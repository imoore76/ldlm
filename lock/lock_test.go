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

func TestLock(t *testing.T) {
	assert := assert.New(t)

	lk := lock.NewLock()
	ctx := context.Background()
	ch := lk.Lock("key", ctx)

	lr := <-ch
	if err, ok := lr.(error); ok {
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
		ch := lk.Lock("otherkey", ctx)
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

	lk := lock.NewLock()

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

	ch := lk.Lock("foo", ctx)

	lr := <-ch
	if err, ok := lr.(error); !ok {
		assert.Fail("Should not have been able to obtain lock")
	} else {
		assert.Equal(err, ErrTimeout)
	}

}

func TestUnlockInvalidKey(t *testing.T) {
	assert := assert.New(t)

	lk := lock.NewLock()
	locked, err := lk.TryLock("key")
	assert.Nil(err)
	assert.True(locked)

	unlocked, err := lk.Unlock("otherkey")
	assert.False(unlocked, "Should not have been able to unlock with different key")
	assert.Equal(err, lock.ErrInvalidLockKey)

}
