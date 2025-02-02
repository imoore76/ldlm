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

TODO: https://pkg.go.dev/golang.org/x/sync/semaphore
*/
package lock

import (
	"context"
	"errors"
	"slices"
)

var (
	ErrInvalidLockKey = errors.New("invalid lock key")
	ErrLockNotLocked  = errors.New("lock is not locked")
)

type Lock struct {
	size   int32
	keys   []string
	mtx    chan struct{}
	keyMtx chan struct{}
	ctx    context.Context
}

func NewLock(ctx context.Context, n int32) *Lock {
	return &Lock{
		size:   n,
		keys:   []string{},
		keyMtx: make(chan struct{}, 1),
		mtx:    make(chan struct{}, n),
		ctx:    ctx,
	}
}

// Size returns the lock's size
func (l *Lock) Size() int32 {
	return l.size
}

// Keys returns the lock's keys
func (l *Lock) Keys() []string {
	l.lockKey()
	defer l.unlockKey()
	keysCopy := make([]string, len(l.keys))
	copy(keysCopy, l.keys)
	return keysCopy
}

// lockKey locks the lock's key for inspection / setting to avoid race
// conditions. It uses a channel instead of a mutex which seems a little more
// performant when profiling
func (l *Lock) lockKey() {
	l.keyMtx <- struct{}{}
}

// lockKey locks the lock's key mutex
func (l *Lock) unlockKey() {
	<-l.keyMtx
}

// Lock blocks until the lock is obtained or is canceled / timed out by context
// and returns a channel with the result. The interface returned will be either
// a struct{} (meaning the lock was acquired) or an error
func (l *Lock) Lock(key string, ctx context.Context) <-chan interface{} {
	rChan := make(chan interface{})
	go func() {
		if l.ctx.Err() != nil {
			rChan <- l.ctx.Err()
			return
		}

		select {
		case l.mtx <- struct{}{}:
			l.lockKey()
			defer l.unlockKey()
			l.keys = append(l.keys, key)
			rChan <- struct{}{}
		case <-ctx.Done(): // Cancelled or timed out
			rChan <- context.Cause(ctx)
		case <-l.ctx.Done():
			rChan <- context.Cause(l.ctx)
		}
	}()
	return rChan
}

// TryLock tries to obtain the lock and immediately fails or succeeds
func (l *Lock) TryLock(key string) (bool, error) {
	// Return context error if it exists
	if l.ctx.Err() != nil {
		return false, l.ctx.Err()
	}

	select {
	case l.mtx <- struct{}{}:
		l.lockKey()
		defer l.unlockKey()
		l.keys = append(l.keys, key)
		return true, nil
	default:
		return false, nil
	}
}

// Unlock surprisingly unlocks the lock
func (l *Lock) Unlock(key string) (bool, error) {
	// Return context error if it exists
	if l.ctx.Err() != nil {
		return false, l.ctx.Err()
	}

	l.lockKey()
	defer l.unlockKey()

	at := slices.Index(l.keys, key)
	if at < 0 {
		return false, ErrInvalidLockKey
	}

	l.keys = slices.Delete(l.keys, at, at+1)
	<-l.mtx
	return true, nil
}
