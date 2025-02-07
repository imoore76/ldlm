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

func TestLocking(t *testing.T) {
	assert := assert.New(t)
	lctx := context.Background()
	lk := lock.NewLock(lctx, 1)
	ctx := context.Background()
	err := lk.Lock("key", ctx)

	if err != nil {
		assert.Fail("Could not obtain lock", err)
		t.FailNow()
	}

	locked, err := lk.TryLock("key")
	assert.Nil(err)
	assert.False(locked, "Locked when lock was held")

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		// Wait for lock
		started := time.Now()
		err := lk.Lock("otherkey", ctx)
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
		unlocked, err := lk.Unlock("key")
		assert.Nil(err)
		assert.True(unlocked, "lock should be unlocked")
		wg.Done()
	}()

	wg.Wait()

	unlocked, err := lk.Unlock("key")
	assert.False(unlocked, "Should not have unlocked with old key")
	assert.Equal(err, lock.ErrInvalidLockKey)

	unlocked, err = lk.Unlock("otherkey")
	assert.True(unlocked, "Should have unlocked")
	assert.Nil(err)

}

func TestLockTimeout(t *testing.T) {
	assert := assert.New(t)

	lctx := context.Background()
	lk := lock.NewLock(lctx, 1)

	locked, err := lk.TryLock("key")
	assert.Nil(err)
	assert.True(locked)

	var ErrTimeout = errors.New("oops!")

	ctx, cancel := context.WithTimeoutCause(
		context.Background(),
		time.Millisecond,
		ErrTimeout,
	)
	defer cancel()
	time.Sleep(100 * time.Millisecond)

	err = lk.Lock("foo", ctx)
	assert.Equal(ErrTimeout, err, "Lock should have timed out")

}

func TestUnlockInvalidKey(t *testing.T) {
	assert := assert.New(t)

	lctx := context.Background()
	lk := lock.NewLock(lctx, 1)
	locked, err := lk.TryLock("key")
	assert.Nil(err)
	assert.True(locked)

	unlocked, err := lk.Unlock("otherkey")
	assert.False(unlocked, "Should not have been able to unlock with different key")
	assert.Equal(err, lock.ErrInvalidLockKey)

}

func TestTryLock_Size(t *testing.T) {
	assert := assert.New(t)

	lctx := context.Background()
	lk := lock.NewLock(lctx, 3)

	locked, err := lk.TryLock("key")
	assert.Nil(err)
	assert.True(locked)

	locked, err = lk.TryLock("key")
	assert.Nil(err)
	assert.True(locked)

	locked, err = lk.TryLock("key")
	assert.Nil(err)
	assert.True(locked)

	locked, err = lk.TryLock("key")
	assert.Nil(err)
	assert.False(locked)

	// Unlocking should make room for one more
	unlocked, err := lk.Unlock("key")
	assert.True(unlocked, "lock should be unlocked")
	assert.Nil(err)

	// One more
	locked, err = lk.TryLock("key")
	assert.Nil(err)
	assert.True(locked)

	// But nothing else
	locked, err = lk.TryLock("key")
	assert.Nil(err)
	assert.False(locked)
}

func TestLock_Size(t *testing.T) {
	assert := assert.New(t)

	lctx := context.Background()
	lk := lock.NewLock(lctx, 3)

	err := lk.Lock("key", lctx)
	if err != nil {
		assert.Fail("Could not obtain lock", err)
		t.FailNow()
	}

	err = lk.Lock("key", lctx)
	if err != nil {
		assert.Fail("Could not obtain lock", err)
		t.FailNow()
	}

	err = lk.Lock("key", lctx)
	if err != nil {
		assert.Fail("Could not obtain lock", err)
		t.FailNow()
	}

	ctx, stopWaiting := context.WithCancelCause(lctx)
	done := make(chan struct{})
	var locked bool
	go func() {
		err = lk.Lock("key", ctx)
		locked = (err == nil)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	stopWaiting(errors.New("stop waiting"))
	<-done
	assert.False(locked)
}
