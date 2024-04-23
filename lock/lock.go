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
)

var (
	ErrInvalidLockKey = errors.New("invalid lock key")
	ErrLockNotLocked  = errors.New("lock is not locked")
)

// Lock represents a lock
type Lock struct {
	key    string
	keyMtx chan struct{}
	mtx    chan struct{}
}

// NewLock initializes and returns a new Lock instance.
func NewLock() *Lock {
	return &Lock{
		keyMtx: make(chan struct{}, 1),
		mtx:    make(chan struct{}, 1),
	}
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
func (l *Lock) Lock(key string, ctx context.Context) chan interface{} {
	rChan := make(chan interface{})
	go func() {
		select {
		case l.mtx <- struct{}{}:
			l.lockKey()
			l.key = key
			l.unlockKey()
			rChan <- struct{}{}
		case <-ctx.Done(): // Cancelled or timed out
			rChan <- context.Cause(ctx)
		}
	}()
	return rChan
}

// TryLock tries to obtain the lock and immediately fails or succeeds
func (l *Lock) TryLock(key string) (bool, error) {
	select {
	case l.mtx <- struct{}{}:
		l.lockKey()
		defer l.unlockKey()
		if l.key != "" {
			return false, nil
		}
		l.key = key
		return true, nil
	default:
		return false, nil
	}
}

// Unlock surprisingly unlocks the lock
func (l *Lock) Unlock(k string) (bool, error) {
	l.lockKey()
	defer l.unlockKey()

	if l.key == "" {
		return false, ErrLockNotLocked
	}

	if l.key == k {
		l.key = ""
		<-l.mtx
		return true, nil
	}

	return false, ErrInvalidLockKey
}
