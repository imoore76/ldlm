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

package locksrv_test

import (
	"context"
	"fmt"
	log "log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/imoore76/go-ldlm/config"
	con "github.com/imoore76/go-ldlm/constants"
	_ "github.com/imoore76/go-ldlm/log"
	pb "github.com/imoore76/go-ldlm/protos"
	lsrv "github.com/imoore76/go-ldlm/server/locksrv"
	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
	rw "github.com/imoore76/go-ldlm/server/locksrv/lockmap/readwriter"
)

var defaultTestOpts = map[string]string{
	"lock_gc_min_idle":     "1h",
	"lock_gc_interval":     "1h",
	"default_lock_timeout": "1h",
}

func getTestConfig(opts map[string]string) (*lsrv.LockSrvConfig, func()) {
	tmpFile, _ := os.CreateTemp("", "ldlm-test-ipc-*")
	tmpFile.Close()
	os.Remove(tmpFile.Name())

	confOptsMap := defaultTestOpts
	for k, v := range opts {
		confOptsMap[k] = v
	}
	confOpts := []string{"--ipc_socket_file", tmpFile.Name()}
	for k, v := range confOptsMap {
		confOpts = append(confOpts, "--"+k, v)
	}
	return config.Configure[lsrv.LockSrvConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix, Args: confOpts},
	), func() { os.Remove(tmpFile.Name()) }
}

func testingServer(conf *lsrv.LockSrvConfig) (clientFactory func() (pb.LDLMClient, func()), closer func()) {
	// from https://medium.com/@3n0ugh/how-to-test-grpc-servers-in-go-ba90fe365a18
	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	ctx := context.Background()
	server, lsStopperFunc := lsrv.New(conf)

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(server),
	)
	pb.RegisterLDLMServer(grpcServer, server)

	// Blocking call
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("error starting server: %v", err)
		}
	}()

	closer = func() {
		log.Warn("Closing test server")
		err := lis.Close()
		if err != nil {
			log.Error("error closing listener: %v", err)
		}
		lsStopperFunc()
		grpcServer.Stop()
	}

	// Factory function for generating clients connected to this server instance
	clientFactory = func() (pb.LDLMClient, func()) {
		conn, err := grpc.DialContext(ctx, "",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			panic(fmt.Sprintf("error connecting to server: %v", err))
		}
		// Return client and a client connection closer
		return pb.NewLDLMClient(conn), func() { conn.Close() }
	}

	return clientFactory, closer
}

func TestLocking(t *testing.T) {
	assert := assert.New(t)

	var client1Key, client2Key string
	c, confCloser := getTestConfig(nil)
	defer confCloser()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	// Make RPC using the context with the metadata.
	client, _ := mkClient()
	res, err := client.Lock(ctx, &pb.LockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.Nil(res.Error)
	assert.True(res.Locked, "Could not obtain lock")
	client1Key = res.Key

	client2, _ := mkClient()
	res, err = client2.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.Nil(res.Error)
	assert.False(res.Locked, "Locked when lock was held")
	client2Key = res.Key

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := client2.Lock(ctx, &pb.LockRequest{
			Name: "testlock",
		})
		assert.Nil(err)
		assert.Nil(res.Error)
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
		res, err := client.Unlock(ctx, &pb.UnlockRequest{
			Name: "testlock",
			Key:  client1Key,
		})
		if err != nil {
			// can't call t.FailNow from goroutine
			panic(fmt.Sprintf("Error unlocking lock %v", err.Error()))
		} else {
			assert.Nil(res.Error)
			assert.True(res.Unlocked, "lock should be unlocked")
		}
		wg.Done()
	}()

	wg.Wait()

	// This should fail because lock is not owned by this client
	ures, err := client.Unlock(ctx, &pb.UnlockRequest{
		Name: "testlock",
		Key:  client1Key,
	})
	assert.Nil(err)
	assert.False(ures.Unlocked, "lock should not be unlocked")
	assert.Equal("invalid lock key", ures.Error.Message)
	assert.Equal(pb.ErrorCode_InvalidLockKey, ures.Error.Code)

	// This should succeed
	ures, err = client2.Unlock(ctx, &pb.UnlockRequest{
		Name: "testlock",
		Key:  client2Key,
	})
	assert.Nil(err)
	assert.Nil(ures.Error)
	assert.True(ures.Unlocked, "lock should be unlocked")

}

func TestClientDisconnectUnlock(t *testing.T) {
	assert := assert.New(t)

	c, confCloser := getTestConfig(nil)
	defer confCloser()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	// client1 obtains "testlock"
	client1, client1Close := mkClient()
	res, err := client1.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.True(res.Locked)

	// client2 obtains "testlock2"
	client2, _ := mkClient()
	res, err = client2.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock2",
	})
	assert.Nil(err)
	assert.True(res.Locked)

	// client3 waits to obtain "testlock"
	client3, _ := mkClient()
	done := make(chan struct{})
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := client3.Lock(ctx, &pb.LockRequest{
			Name: "testlock",
		})
		assert.Nil(err)
		assert.Nil(res.Error)
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
	client1Close()

	// Wait for client3 to obtain "testlock", which it had been
	// blocking on
	<-done

	// client1 disconnected and its locks have should be unlocked. Now client3
	// holds "testlock". "testlock2" should still be held by client2 as the
	// client connection cleanup should be restricted to locks held by client1
	res, err = client3.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock2",
	})
	assert.Nil(err)
	assert.False(res.Locked, "testlock2 should be held by client2")

}

func TestUnlockNotLocked(t *testing.T) {
	assert := assert.New(t)

	c, confCloser := getTestConfig(nil)
	defer confCloser()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	client1, _ := mkClient()
	res, err := client1.Unlock(ctx, &pb.UnlockRequest{
		Name: "testlock",
		Key:  "somekey",
	})
	assert.Nil(err)
	assert.False(res.Unlocked)
	assert.NotNil(res.Error)
	if res.Error != nil {
		assert.Equal(pb.ErrorCode_LockDoesNotExist, res.Error.Code)
		assert.Equal("a lock by that name does not exist", res.Error.Message)
	}
}

func TestLockEmptyName(t *testing.T) {
	assert := assert.New(t)

	c, confCloser := getTestConfig(nil)
	defer confCloser()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	client1, _ := mkClient()
	res, err := client1.Lock(ctx, &pb.LockRequest{
		Name: "",
	})
	assert.Nil(err)
	assert.False(res.Locked)
	assert.NotNil(res.Error)
	if res.Error != nil {
		assert.Equal(pb.ErrorCode_Unknown, res.Error.Code)
		assert.Equal("lock name cannot be empty", res.Error.Message)
	}
}

func TestLockWaitTimeout(t *testing.T) {
	assert := assert.New(t)

	c, confCloser := getTestConfig(nil)
	defer confCloser()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	// Client1 holds lock
	client1, _ := mkClient()
	res, err := client1.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)

	// Client2 calls WaitLock with a timeout
	client2, _ := mkClient()
	var to uint32 = 1
	res2, err := client2.Lock(ctx, &pb.LockRequest{
		Name:               "testlock",
		WaitTimeoutSeconds: &to,
	})

	// The result is that a timeout error is returned
	assert.Nil(err)
	assert.False(res2.Locked)
	assert.NotNil(res2.Error)
	if res2.Error != nil {
		assert.Equal(pb.ErrorCode_LockWaitTimeout, res2.Error.Code)
		assert.Equal("timeout waiting to acquire lock", res2.Error.Message)
	}
}

func TestLockWaiterDisconnects(t *testing.T) {
	assert := assert.New(t)

	var client1Key string
	c, confCloser := getTestConfig(nil)
	defer confCloser()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	// client1 obtains lock
	client1, _ := mkClient()
	res, err := client1.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)
	client1Key = res.Key

	// client2 waits on lock
	client2, c2Close := mkClient()
	go func() {
		client2.Lock(ctx, &pb.LockRequest{
			Name: "testlock",
		})
	}()
	time.Sleep(2 * time.Second)

	// client2 disconnects
	c2Close()

	// client1 unlocks lock
	res2, err := client1.Unlock(ctx, &pb.UnlockRequest{
		Name: "testlock",
		Key:  client1Key,
	})
	assert.Nil(err)
	assert.True(res2.Unlocked)

	// Let other goroutines do their things
	time.Sleep(1 * time.Second)

	// client3 can obtain lock
	// (lock is not deadlocked by being handed out to a disconnected client)
	client3, _ := mkClient()
	res3, err := client3.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.True(res3.Locked)
	assert.Nil(res3.Error)

}

func TestLockTimerTimeout(t *testing.T) {
	assert := assert.New(t)

	c, confCloser := getTestConfig(nil)
	defer confCloser()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	// Client1 holds lock
	client1, _ := mkClient()
	client2, _ := mkClient()

	timeout := uint32(3)
	res, err := client1.TryLock(ctx, &pb.TryLockRequest{
		Name:               "testlock",
		LockTimeoutSeconds: &timeout,
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)
	lockKey := res.Key

	res, err = client2.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.False(res.Locked, "Lock should not have been obtained")

	wait := make(chan struct{})
	var newLockKey string
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := client2.Lock(ctx, &pb.LockRequest{
			Name: "testlock",
		})
		assert.Nil(err)
		assert.Nil(res.Error)
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

	ures, err := client1.Unlock(ctx, &pb.UnlockRequest{
		Name: "testlock",
		Key:  lockKey,
	})
	assert.Nil(err)
	assert.False(ures.Unlocked, "lock should not be unlocked")
	assert.Equal("invalid lock key", ures.Error.Message)
	assert.Equal(pb.ErrorCode_InvalidLockKey, ures.Error.Code)

	ures, err = client2.Unlock(ctx, &pb.UnlockRequest{
		Name: "testlock",
		Key:  newLockKey,
	})
	assert.Nil(err)
	assert.True(ures.Unlocked, "lock should be unlocked")
}

func TestLockTimerRefresh(t *testing.T) {
	assert := assert.New(t)

	c, confCloser := getTestConfig(nil)
	defer confCloser()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	// Client1 holds lock
	client1, _ := mkClient()
	client2, _ := mkClient()

	timeout := uint32(1)
	res, err := client1.TryLock(ctx, &pb.TryLockRequest{
		Name:               "testlock",
		LockTimeoutSeconds: &timeout,
	})
	assert.Nil(err)
	assert.True(res.Locked, "Client1 should have obtained lock")
	assert.Nil(res.Error)
	lockKey := res.Key

	refreshStop := make(chan struct{})
	refreshStopped := make(chan struct{})
	go func() {
		// Wait for lock
		for {
			time.Sleep(500 * time.Millisecond)
			select {
			case <-refreshStop:
				refreshStopped <- struct{}{}
				return
			default:
				res, err := client1.RefreshLock(ctx, &pb.RefreshLockRequest{
					Name:               "testlock",
					Key:                lockKey,
					LockTimeoutSeconds: 1,
				})
				assert.Nil(err)
				assert.True(res.Locked, "Client1 should have refreshed lock")
				assert.Nil(res.Error)
			}
		}
	}()

	res, err = client2.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.False(res.Locked, "Lock should not have been obtained by client2")

	var newLockKey string
	lockObtained := make(chan struct{})
	go func() {
		// Wait for lock
		started := time.Now()
		res, err := client2.Lock(ctx, &pb.LockRequest{
			Name: "testlock",
		})
		assert.Nil(err)
		assert.Nil(res.Error)
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
	refreshStop <- struct{}{}
	<-refreshStopped

	<-lockObtained

	ures, err := client1.Unlock(ctx, &pb.UnlockRequest{
		Name: "testlock",
		Key:  lockKey,
	})

	assert.Nil(err)
	assert.False(ures.Unlocked, "lock should not be unlocked")
	assert.Equal("invalid lock key", ures.Error.Message)
	assert.Equal(pb.ErrorCode_InvalidLockKey, ures.Error.Code)

	ures, err = client2.Unlock(ctx, &pb.UnlockRequest{
		Name: "testlock",
		Key:  newLockKey,
	})
	assert.Nil(err)
	assert.True(ures.Unlocked, "lock should be unlocked")

}

func TestLoadLocks(t *testing.T) {
	assert := assert.New(t)

	tmpfile, err := os.CreateTemp("", "ldlm-test-*")
	assert.NoError(err)

	c, confCloser := getTestConfig(map[string]string{
		"state_file":           tmpfile.Name(),
		"default_lock_timeout": "2s",
	})
	defer confCloser()
	assert.Equal(c.LockMapConfig.StateFile, tmpfile.Name())

	rwr, err := rw.New(&c.LockMapConfig.Config)
	assert.NoError(err)

	rwr.Write(map[string][]cl.ClientLock{
		"testkey": {*cl.New("testlock", "testkey")},
	})
	rwr.Close()

	ctx := context.Background()
	mkClient, closer := testingServer(c)
	defer closer()

	client, _ := mkClient()
	res, err := client.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.Nil(res.Error)
	assert.False(res.Locked, "Locked when lock was held")

	time.Sleep(2 * time.Second)

	res, err = client.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.Nil(res.Error)
	assert.True(res.Locked, "Could not obtain lock")

}
