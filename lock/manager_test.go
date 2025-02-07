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

	"github.com/imoore76/ldlm/lock"
	"github.com/stretchr/testify/assert"
)

func TestLockGc(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(2, time.Second, time.Second)
	defer cancelFunc()

	err := lm.Lock("mylock", "key", 1, context.Background())
	assert.Nil(err)

	if err != nil {
		assert.Fail("Could not obtain lock: %v", err.Error())
		t.FailNow()
	}

	locked, err := lm.TryLock("mylock", "key", 1)
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

func TestShutdown_ManagerLock(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(2, time.Hour, time.Hour)
	cancelFunc()

	time.Sleep(time.Second)

	err := lm.Lock("mylock", "key", 1, context.Background())
	assert.ErrorIs(lock.ErrManagerShutdown, err)
}

func TestShutdown_ManagerTryLock(t *testing.T) {
	assert := assert.New(t)
	lm, cancelFunc := lock.NewManager(2, time.Hour, time.Hour)
	cancelFunc()

	time.Sleep(time.Second)

	_, err := lm.TryLock("mylock", "key", 1)
	assert.ErrorIs(lock.ErrManagerShutdown, err)
}

func TestManagedLock(t *testing.T) {
	assert := assert.New(t)

	ctx := context.Background()

	lm, cancelFunc := lock.NewManager(2, time.Hour, time.Hour)
	defer cancelFunc()

	err := lm.Lock("mylock", "key", 1, ctx)
	assert.Nil(err)

	if err != nil {
		assert.Fail("Could not obtain lock: %v", err.Error())
		t.FailNow()
	}

	locked, err := lm.TryLock("mylock", "key", 1)
	assert.Nil(err)
	assert.False(locked, "Locked when lock was held")

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		// Wait for lock
		started := time.Now()
		err := lm.Lock("mylock", "otherkey", 1, ctx)
		assert.Nil(err)

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

	lm, cancelFunc := lock.NewManager(2, time.Hour, time.Hour)
	defer cancelFunc()

	locked, err := lm.TryLock("lock", "key", 1)
	assert.Nil(err)
	assert.True(locked)

	var ErrTimeout = errors.New("oops!")

	ctx, cancel := context.WithTimeoutCause(
		context.Background(),
		time.Millisecond,
		ErrTimeout,
	)
	defer cancel()

	err = lm.Lock("lock", "", 1, ctx)
	assert.Equal(ErrTimeout, err)

}

func TestManager_UnlockInvalidKey(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(2, time.Hour, time.Hour)
	defer cancelFunc()

	locked, err := lm.TryLock("lock", "key", 1)
	assert.Nil(err)
	assert.True(locked)

	unlocked, err := lm.Unlock("lock", "otherkey")
	assert.False(unlocked, "Should not have been able to unlock with different key")
	assert.Equal(err, lock.ErrInvalidLockKey)
}

func TestManager_UnlockNotFound(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(2, time.Hour, time.Hour)
	defer cancelFunc()

	unlocked, err := lm.Unlock("lock", "otherkey")
	assert.False(unlocked, "Should not have been able to unlock a lock that doesn't exist")
	assert.NotNil(err)
	if err != nil {
		assert.Equal(err, lock.ErrLockDoesNotExist)
	}

}

func TestManager_SizeMismatch(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(2, time.Hour, time.Hour)
	defer cancelFunc()

	locked, err := lm.TryLock("lock", "key", 4)
	assert.Nil(err)
	assert.True(locked)

	locked, err = lm.TryLock("lock", "asdf", 5)
	assert.ErrorIs(lock.ErrLockSizeMismatch, err)
	assert.False(locked)

}

func TestManager_InvalidSize(t *testing.T) {
	assert := assert.New(t)

	lm, cancelFunc := lock.NewManager(2, time.Hour, time.Hour)
	defer cancelFunc()

	err := lm.Lock("lock", "key", -2, context.Background())
	assert.ErrorIs(lock.ErrInvalidLockSize, err)

	err = lm.Lock("lock", "key", 0, context.Background())
	assert.ErrorIs(lock.ErrInvalidLockSize, err)

}
