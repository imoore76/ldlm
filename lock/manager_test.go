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

package lock_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/imoore76/go-ldlm/lock"
	"github.com/stretchr/testify/assert"
)

func TestLockGc(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(time.Second, time.Second)
	defer cancelFunc()

	ch, err := lm.Lock("mylock", "key", context.Background())
	assert.Nil(err)

	lr := <-ch
	if err, ok := lr.(error); ok {
		assert.Fail("Could not obtain lock: %v", err.Error())
		t.FailNow()
	}

	locked, err := lm.TryLock("mylock", "key")
	assert.Nil(err)
	assert.False(locked, "Locked when lock was held")

	// Wait for 2x the GC interval
	time.Sleep(2 * time.Second)

	assert.Equal(len(lm.Locks()), 1, "There should still be exactly one lock")

	unlocked, err := lm.Unlock("mylock", "key")
	assert.Nil(err)

	assert.True(unlocked, "The lock should be unlocked")

	// Wait for 2x the GC interval
	time.Sleep(2 * time.Second)

	assert.Equal(len(lm.Locks()), 0, "The lock should have been garbage collected")

}

func TestShutdownManagerLock(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(time.Hour, time.Hour)
	cancelFunc()

	time.Sleep(time.Second)

	defer func() {
		r := recover()
		assert.NotNil(r)
		assert.Equal(r, "Attempted to get lock from a stopped Manager")
	}()
	lm.Lock("mylock", "key", context.Background())
}

func TestShutdownManagerTryLock(t *testing.T) {
	assert := assert.New(t)
	lm, cancelFunc := lock.NewManager(time.Hour, time.Hour)
	cancelFunc()

	time.Sleep(time.Second)

	defer func() {
		r := recover()
		assert.NotNil(r)
		assert.Equal(r, "Attempted to get lock from a stopped Manager")
	}()
	lm.TryLock("mylock", "key")
}

func TestManagedLock(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()

	lm, cancelFunc := lock.NewManager(time.Hour, time.Hour)
	defer cancelFunc()

	ch, err := lm.Lock("mylock", "key", ctx)
	assert.Nil(err)

	lr := <-ch

	if err, ok := lr.(error); ok {
		assert.Fail("Could not obtain lock: %v", err.Error())
		t.FailNow()
	}

	locked, err := lm.TryLock("mylock", "key")
	assert.Nil(err)
	assert.False(locked, "Locked when lock was held")

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		// Wait for lock
		started := time.Now()
		ch, err := lm.Lock("mylock", "otherkey", ctx)
		assert.Nil(err)
		<-ch
		assert.GreaterOrEqual(
			time.Since(started),
			(2 * time.Second),
			"It should have been 2 or more seconds waiting to acquire lock",
		)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		time.Sleep(2 * time.Second)
		unlocked, err := lm.Unlock("mylock", "key")
		assert.Nil(err)
		assert.True(unlocked, "lock should be unlocked")
		wg.Done()
	}()

	wg.Wait()

	unlocked, err := lm.Unlock("mylock", "key")
	assert.False(unlocked, "Should not have unlocked with old key")
	assert.Equal(err, lock.ErrInvalidLockKey)

	unlocked, err = lm.Unlock("mylock", "otherkey")
	assert.True(unlocked, "Should have unlocked")
	assert.Nil(err)

}

func TestWaitManagedLockTimeout(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(time.Hour, time.Hour)
	defer cancelFunc()

	locked, err := lm.TryLock("lock", "key")
	assert.Nil(err)
	assert.True(locked)

	var ErrTimeout = errors.New("oops!")

	ctx, cancel := context.WithTimeoutCause(
		context.Background(),
		time.Millisecond,
		ErrTimeout,
	)
	defer cancel()

	ch, err := lm.Lock("lock", "", ctx)
	assert.Nil(err)

	lr := <-ch
	if err, ok := lr.(error); !ok {
		assert.Fail("Should not have been able to obtain lock")
	} else {
		assert.Equal(err, ErrTimeout)
	}

}

func TestManagerUnlockInvalidKey(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(time.Hour, time.Hour)
	defer cancelFunc()

	locked, err := lm.TryLock("lock", "key")
	assert.Nil(err)
	assert.True(locked)

	unlocked, err := lm.Unlock("lock", "otherkey")
	assert.False(unlocked, "Should not have been able to unlock with different key")
	assert.Equal(err, lock.ErrInvalidLockKey)
}

func TestManagerUnlockNotFound(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(time.Hour, time.Hour)
	defer cancelFunc()

	unlocked, err := lm.Unlock("lock", "otherkey")
	assert.False(unlocked, "Should not have been able to unlock a lock that doesn't exist")
	assert.NotNil(err)
	if err != nil {
		assert.Equal(err, lock.ErrLockDoesNotExist)
	}

}
