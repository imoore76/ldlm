// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	config "github.com/imoore76/configurature"
	"github.com/stretchr/testify/assert"

	"github.com/imoore76/go-ldlm/constants"
	"github.com/imoore76/go-ldlm/lock"
	_ "github.com/imoore76/go-ldlm/log"
	"github.com/imoore76/go-ldlm/server"
	cl "github.com/imoore76/go-ldlm/server/clientlock"
	"github.com/imoore76/go-ldlm/server/session/store"
)

var defaultTestOpts = map[string]string{
	"lock_gc_min_idle":     "1h",
	"lock_gc_interval":     "1h",
	"default_lock_timeout": "1h",
}

type testClient struct {
	server *server.LockServer
	ctx    context.Context
	close  func()
}

func (t *testClient) Close() {
	t.server.DestroySession(t.ctx)
	t.close()
}

func newTestClient(server *server.LockServer) *testClient {
	_, ctx := server.CreateSession(context.Background(), nil)
	ctx, cancel := context.WithCancel(ctx)
	return &testClient{
		ctx:    ctx,
		server: server,
		close:  cancel,
	}
}

func getTestConfig(opts map[string]string) *server.LockServerConfig {
	confOptsMap := defaultTestOpts
	for k, v := range opts {
		confOptsMap[k] = v
	}
	confOpts := []string{"--ipc_socket_file", ""}
	for k, v := range confOptsMap {
		confOpts = append(confOpts, "--"+k, v)
	}
	return config.Configure[server.LockServerConfig](
		&config.Options{EnvPrefix: constants.TestConfigEnvPrefix, Args: confOpts},
	)
}

func TestLocking(t *testing.T) {
	assert := assert.New(t)

	var client1Key, client2Key string
	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	client := newTestClient(s)
	res, err := s.Lock(client.ctx, "testlock", nil, nil, nil)
	assert.Nil(err)
	assert.True(res.Locked, "Could not obtain lock")
	client1Key = res.Key

	client2 := newTestClient(s)
	res, err = s.TryLock(client2.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.False(res.Locked, "Locked when lock was held")
	client2Key = res.Key

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := s.Lock(client2.ctx, "testlock", nil, nil, nil)
		assert.Nil(err)
		assert.True(res.Locked, "Lock should have been obtained")
		assert.GreaterOrEqual(
			time.Since(started),
			(2 * time.Second),
			"It should have been 2 or more seconds waiting to acquire lock",
		)
		client2Key = res.Key
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		time.Sleep(2 * time.Second)
		unlocked, err := s.Unlock(client.ctx, "testlock", client1Key)
		if err != nil {
			// can't call t.FailNow from goroutine
			panic(fmt.Sprintf("Error unlocking lock %v", err.Error()))
		} else {
			assert.Nil(nil)
			assert.True(unlocked, "lock should be unlocked")
		}
		wg.Done()
	}()

	wg.Wait()

	// This should fail because the key is incorrect
	unlocked, err := s.Unlock(client.ctx, "testlock", client1Key)
	assert.NotNil(err)
	assert.False(unlocked, "lock should not be unlocked")
	assert.Equal("invalid lock key", err.Error())

	// This should succeed
	unlocked, err = s.Unlock(client2.ctx, "testlock", client2Key)
	assert.Nil(err)
	assert.Nil(err)
	assert.True(unlocked, "lock should be unlocked")

}

func TestLocking_Size(t *testing.T) {
	assert := assert.New(t)

	sz := new(int32)
	*sz = 3

	var client1Key, client2Key string
	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	client := newTestClient(s)
	res, err := s.Lock(client.ctx, "testlock", sz, nil, nil)
	assert.Nil(err)
	assert.True(res.Locked, "Could not obtain lock")

	res, err = s.TryLock(client.ctx, "testlock", sz, nil)
	assert.Nil(err)
	assert.True(res.Locked, "Could not obtain lock")

	res, err = s.TryLock(client.ctx, "testlock", sz, nil)
	assert.Nil(err)
	assert.True(res.Locked, "Could not obtain lock")

	client1Key = res.Key

	client2 := newTestClient(s)
	res, err = s.TryLock(client2.ctx, "testlock", sz, nil)
	assert.Nil(err)
	assert.False(res.Locked, "Locked when lock was held")
	client2Key = res.Key

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := s.Lock(client2.ctx, "testlock", sz, nil, nil)
		assert.Nil(err)
		assert.True(res.Locked, "Lock should have been obtained")
		assert.GreaterOrEqual(
			time.Since(started),
			(2 * time.Second),
			"It should have been 2 or more seconds waiting to acquire lock",
		)
		client2Key = res.Key
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		time.Sleep(2 * time.Second)
		unlocked, err := s.Unlock(client.ctx, "testlock", client1Key)
		if err != nil {
			// can't call t.FailNow from goroutine
			panic(fmt.Sprintf("Error unlocking lock %v", err.Error()))
		} else {
			assert.Nil(nil)
			assert.True(unlocked, "lock should be unlocked")
		}
		wg.Done()
	}()

	wg.Wait()

	// This should fail because the key is incorrect
	unlocked, err := s.Unlock(client.ctx, "testlock", client1Key)
	assert.NotNil(err)
	assert.False(unlocked, "lock should not be unlocked")
	assert.Equal("invalid lock key", err.Error())

	// This should succeed
	unlocked, err = s.Unlock(client2.ctx, "testlock", client2Key)
	assert.Nil(err)
	assert.Nil(err)
	assert.True(unlocked, "lock should be unlocked")
}

func TestClientDisconnectUnlock(t *testing.T) {
	assert := assert.New(t)

	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	// client1 obtains "testlock"
	client1 := newTestClient(s)
	res, err := s.TryLock(client1.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.True(res.Locked)

	// client2 obtains "testlock2"
	client2 := newTestClient(s)
	res, err = s.TryLock(client2.ctx, "testlock2", nil, nil)
	assert.Nil(err)
	assert.True(res.Locked)

	// client3 waits to obtain "testlock"
	client3 := newTestClient(s)
	done := make(chan struct{})
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := s.Lock(client3.ctx, "testlock", nil, nil, nil)
		assert.Nil(err)
		assert.True(res.Locked, "Lock should have been obtained")
		assert.GreaterOrEqual(
			time.Since(started),
			(2 * time.Second),
			"It should have been 2 or more seconds waiting to acquire lock",
		)
		done <- struct{}{}
	}()

	// Wait 2 seconds and close client1's connection
	time.Sleep(2 * time.Second)
	client1.Close()

	// Wait for client3 to obtain "testlock", which it had been
	// blocking on
	<-done

	// client1 disconnected and its locks have should be unlocked. Now client3
	// holds "testlock". "testlock2" should still be held by client2 as the
	// client connection cleanup should be restricted to locks held by client1
	res, err = s.TryLock(client3.ctx, "testlock2", nil, nil)
	assert.Nil(err)
	assert.False(res.Locked, "testlock2 should be held by client2")
}

func TestUnlockNotLocked(t *testing.T) {
	assert := assert.New(t)

	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	client1 := newTestClient(s)
	unlocked, err := s.Unlock(client1.ctx, "testlock", "somekey")
	assert.NotNil(err)
	assert.False(unlocked)
	if err != nil {
		assert.ErrorIs(err, lock.ErrLockDoesNotExist)
	}
}

func TestLockEmptyName(t *testing.T) {
	assert := assert.New(t)

	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	client1 := newTestClient(s)
	res, err := s.Lock(client1.ctx, "", nil, nil, nil)
	assert.NotNil(err)
	assert.False(res.Locked)
	if err != nil {
		assert.ErrorIs(err, server.ErrEmptyName)
	}

	res, err = s.TryLock(client1.ctx, "", nil, nil)
	assert.NotNil(err)
	assert.False(res.Locked)
	if err != nil {
		assert.ErrorIs(err, server.ErrEmptyName)
	}

}

func TestLockWaitTimeout(t *testing.T) {
	assert := assert.New(t)

	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	// Client1 holds lock
	client1 := newTestClient(s)
	res, err := s.TryLock(client1.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.True(res.Locked)

	// Client2 calls WaitLock with a timeout
	client2 := newTestClient(s)
	var to int32 = 1
	res2, err := s.Lock(client2.ctx, "testlock", nil, nil, &to)

	// The result is that a timeout error is returned
	assert.NotNil(err)
	assert.False(res2.Locked)
	if err != nil {
		assert.ErrorIs(err, server.ErrLockWaitTimeout)
	}
}

func TestLockWaiterDisconnects(t *testing.T) {
	assert := assert.New(t)

	var client1Key string
	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	// client1 obtains lock
	client1 := newTestClient(s)
	res, err := s.TryLock(client1.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.True(res.Locked)
	client1Key = res.Key

	// client2 waits on lock
	client2 := newTestClient(s)
	go func() {
		s.Lock(client2.ctx, "testlock", nil, nil, nil)
	}()
	time.Sleep(2 * time.Second)

	// client2 disconnects
	client2.Close()

	// client1 unlocks lock
	unlocked, err := s.Unlock(client1.ctx, "testlock", client1Key)
	assert.Nil(err)
	assert.True(unlocked)

	// Let other goroutines do their things
	time.Sleep(1 * time.Second)

	// client3 can obtain lock
	// (lock is not deadlocked by being handed out to a disconnected client)
	client3 := newTestClient(s)
	res3, err := s.TryLock(client3.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.True(res3.Locked)
}

func TestLockTimerTimeout(t *testing.T) {
	assert := assert.New(t)

	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	// Client1 holds lock
	client1 := newTestClient(s)
	client2 := newTestClient(s)

	timeout := int32(3)
	res, err := s.TryLock(client1.ctx, "testlock", nil, &timeout)
	assert.Nil(err)
	assert.True(res.Locked)
	lockKey := res.Key

	res, err = s.TryLock(client2.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.False(res.Locked, "Lock should not have been obtained")

	wait := make(chan struct{})
	var newLockKey string
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := s.Lock(client2.ctx, "testlock", nil, nil, nil)
		assert.Nil(err)
		assert.True(res.Locked, "Lock should have been obtained")
		assert.GreaterOrEqual(
			time.Since(started),
			(2 * time.Second),
			"It should have been 2 or more seconds waiting to acquire lock",
		)
		newLockKey = res.Key
		wait <- struct{}{}
	}()

	<-wait

	unlocked, err := s.Unlock(client1.ctx, "testlock", lockKey)
	assert.NotNil(err)
	assert.False(unlocked, "lock should not be unlocked")
	assert.ErrorIs(err, lock.ErrInvalidLockKey)

	unlocked, err = s.Unlock(client2.ctx, "testlock", newLockKey)
	assert.Nil(err)
	assert.True(unlocked, "lock should be unlocked")
}

func TestLockTimerRenew(t *testing.T) {
	assert := assert.New(t)

	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	// Client1 holds lock
	client1 := newTestClient(s)
	client2 := newTestClient(s)

	timeout := int32(1)
	res, err := s.TryLock(client1.ctx, "testlock", nil, &timeout)
	assert.Nil(err)
	assert.True(res.Locked, "Client1 should have obtained lock")
	lockKey := res.Key

	renewStop := make(chan struct{})
	renewStopped := make(chan struct{})
	go func() {
		// Wait for lock
		for {
			time.Sleep(500 * time.Millisecond)
			select {
			case <-renewStop:
				renewStopped <- struct{}{}
				return
			default:
				res, err := s.Renew(client1.ctx, "testlock", lockKey, 1)
				assert.Nil(err)
				assert.True(res.Locked, "Client1 should have renewed lock")
			}
		}
	}()

	res, err = s.TryLock(client2.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.False(res.Locked, "Lock should not have been obtained by client2")

	var newLockKey string
	lockObtained := make(chan struct{})
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := s.Lock(client2.ctx, "testlock", nil, nil, nil)
		assert.Nil(err)
		assert.True(res.Locked, "Lock should have been obtained by client2")
		assert.GreaterOrEqual(
			time.Since(started),
			(3 * time.Second),
			"It should have been 3 or more seconds waiting to acquire lock",
		)
		newLockKey = res.Key
		lockObtained <- struct{}{}
	}()

	time.Sleep(3 * time.Second)
	renewStop <- struct{}{}
	<-renewStopped

	<-lockObtained

	unlocked, err := s.Unlock(client1.ctx, "testlock", lockKey)

	assert.NotNil(err)
	assert.False(unlocked, "lock should not be unlocked")
	assert.ErrorIs(err, lock.ErrInvalidLockKey)

	unlocked, err = s.Unlock(client2.ctx, "testlock", newLockKey)
	assert.Nil(err)
	assert.True(unlocked, "lock should be unlocked")

}

func TestLoadLocks(t *testing.T) {
	assert := assert.New(t)

	tmpfile, err := os.CreateTemp("", "ldlm-test-*")
	assert.NoError(err)

	c := getTestConfig(map[string]string{
		"state_file": tmpfile.Name(),
	})
	c.DefaultLockTimeout = 2 * time.Second
	assert.Equal(c.StateFile, tmpfile.Name())

	rwr, err := store.New(c.StateFile)
	assert.NoError(err)

	rwr.Write(map[string][]cl.Lock{
		"testkey": {cl.New("testlock", "testkey", 1)},
	})
	rwr.Close()

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	client := newTestClient(s)
	res, err := s.TryLock(client.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.False(res.Locked, "Locked when lock was held")

	time.Sleep(2500 * time.Millisecond)

	res, err = s.TryLock(client.ctx, "testlock", nil, nil)
	assert.Nil(err)
	assert.True(res.Locked, "Could not obtain lock")

}

func TestLocksMethod_Empty(t *testing.T) {
	assert := assert.New(t)

	c := getTestConfig(map[string]string{
		"state_file": "",
	})

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	assert.Equal([]cl.Lock{}, s.Locks())

}

func TestLocksMethod_Some(t *testing.T) {
	assert := assert.New(t)

	c := getTestConfig(nil)

	s, cancelFunc, err := server.New(c)
	defer cancelFunc()
	assert.Nil(err)

	client1 := newTestClient(s)
	sz := new(int32)
	*sz = 2
	lr1, err := s.TryLock(client1.ctx, "testlock", sz, nil)
	assert.Nil(err)
	*sz = 5
	lr2, err := s.TryLock(client1.ctx, "testlock2", sz, nil)
	assert.Nil(err)

	assert.Equal([]cl.Lock{
		cl.New("testlock", lr1.Key, 2),
		cl.New("testlock2", lr2.Key, 5),
	}, s.Locks())
}
