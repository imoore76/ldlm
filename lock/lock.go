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

/*
This file contains the Lock struct definition and its methods. Lock objects
use channels to lock / unlock which allows for implementing a timeout when
waiting to acquire a lock.
*/
package lock

import (
	"context"
	"errors"
	"slices"
	"sync"

	sem "golang.org/x/sync/semaphore"
)

var (
	ErrInvalidLockKey = errors.New("invalid lock key")
	ErrLockNotLocked  = errors.New("lock is not locked")
)

type Lock struct {
	size   int32
	keys   []string
	sw     sem.Weighted
	keyMtx sync.Mutex
	mgrCtx context.Context // Lock manager context
}

// NewLock returns a lock object with the given size.
func NewLock(mgrCtx context.Context, n int32) *Lock {
	return &Lock{
		size:   n,
		keys:   []string{},
		sw:     *sem.NewWeighted(int64(n)),
		mgrCtx: mgrCtx,
	}
}

// Size returns the lock's size.
func (l *Lock) Size() int32 {
	return l.size
}

// Keys returns the lock's keys.
func (l *Lock) Keys() []string {
	l.keyMtx.Lock()
	defer l.keyMtx.Unlock()
	return slices.Clone(l.keys)
}

// lockKey locks the lock's keys for inspection / setting to avoid race
// conditions.
func (l *Lock) lockKeys() {
	l.keyMtx.Lock()
}

// lockKey locks the lock's key mutex
func (l *Lock) unlockKeys() {
	l.keyMtx.Unlock()
}

// Lock blocks until the lock is obtained or is canceled / timed out by context
// and an error that will be either be nil (meaning the lock was acquired) or
// the error that occurred.
func (l *Lock) Lock(key string, ctx context.Context) error {

	lockCtx, cancel := context.WithCancelCause(ctx)
	defer context.AfterFunc(l.mgrCtx, func() {
		cancel(context.Cause(l.mgrCtx))
	})()

	err := l.sw.Acquire(lockCtx, 1)
	if err != nil {
		if l.mgrCtx.Err() != nil {
			err = context.Cause(l.mgrCtx)
		} else if lockCtx.Err() != nil {
			err = context.Cause(lockCtx)
		}
		return err
	}
	l.addKey(key)
	return nil
}

// TryLock tries to obtain the lock and immediately fails or succeeds.
func (l *Lock) TryLock(key string) (bool, error) {
	select {
	case <-l.mgrCtx.Done():
		// Lock manager shutdown
		return false, context.Cause(l.mgrCtx)
	default:
		if l.sw.TryAcquire(1) {
			l.addKey(key)
			return true, nil
		}
		return false, nil
	}
}

// Unlock surprisingly unlocks the lock.
func (l *Lock) Unlock(key string) (bool, error) {
	// Return context error if it exists
	select {
	case <-l.mgrCtx.Done():
		// Lock manager shutdown
		return false, context.Cause(l.mgrCtx)
	default:
	}

	var err error
	removed := l.removeKey(key)
	if !removed {
		err = ErrInvalidLockKey
	} else {
		// Attempt to release the lock
		l.sw.Release(1)
	}
	return removed, err
}

// addKey adds a key to the lock's keys.
func (l *Lock) addKey(key string) {
	l.keyMtx.Lock()
	defer l.keyMtx.Unlock()
	l.keys = append(l.keys, key)
}

// removeKey removes a key from the lock's keys. It returns false if the key
// does not exist.
func (l *Lock) removeKey(key string) bool {
	l.keyMtx.Lock()
	defer l.keyMtx.Unlock()

	at := slices.Index(l.keys, key)
	if at < 0 {
		return false
	}

	l.keys = slices.Delete(l.keys, at, at+1)
	return true
}
