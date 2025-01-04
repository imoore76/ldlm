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
This file contains the Manager and ManagedLock struct definitions and methods
along with their helper functions.
*/
package lock

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"strings"
	"sync"
	"time"
)

var (
	ErrLockDoesNotExist = errors.New("a lock by that name does not exist")
	ErrManagerShutdown  = errors.New("lock manager is shutdown")
	ErrLockSizeMismatch = errors.New("lock size mismatch")
	ErrInvalidLockSize  = errors.New("lock size must be greater than 0")
)

// LockInfo is a struct that contains only basic information about a lock
type LockInfo struct {
	Name         string
	Keys         string
	LastAccessed time.Time
	Locked       bool
	Size         int
}

// A ManagedLock is a lock with a some record keeping fields for
// garbage collection
type ManagedLock struct {
	Name         string
	lastAccessed time.Time
	deleted      bool
	*Lock
}

// NewManagedLock returns a managed lock object with the given name
func NewManagedLock(name string, ctx context.Context, size int32) *ManagedLock {
	return &ManagedLock{
		Name:         name,
		lastAccessed: time.Now(),
		Lock:         NewLock(ctx, size),
	}
}

// A lock shard is a shard of managed locks
type lockShard struct {
	locks map[string]*ManagedLock
	sync.RWMutex
}

// A Manager polices access to a group of locks addressable by their names
type Manager struct {
	shards        []*lockShard
	stopCh        chan struct{}
	isShutdown    bool
	shutdownMtx   sync.RWMutex
	cancelCauseFn context.CancelCauseFunc
	ctx           context.Context
}

// NewManager creates a new Manager with the given garbage collection interval, minimum idle time,
// and logger, and returns a pointer to the Manager.
//
// Parameters:
//   - ctx context.Context: the context for the Manager
//   - shards uint32: the number of lock shards to use
//   - gcInterval time.Duration: the interval for lock garbage collection
//   - gcMinIdle time.Duration: the minimum time a lock must be idle before being considered for
//     garbage collection
//
// Returns:
// - *Manager: a pointer to the newly created Manager
func NewManager(shards uint32, gcInterval time.Duration, gcMinIdle time.Duration) (*Manager, func()) {

	ctx, cancelCauseFn := context.WithCancelCause(context.Background())

	m := &Manager{
		stopCh:        make(chan struct{}),
		shutdownMtx:   sync.RWMutex{},
		cancelCauseFn: cancelCauseFn,
		ctx:           ctx,
	}
	if shards == 0 {
		shards = 1
	}
	lockShards := make([]*lockShard, shards)
	for i := uint32(0); i < shards; i++ {
		// Create a new lock shard
		lockShards[i] = &lockShard{locks: make(map[string]*ManagedLock)}
	}
	m.shards = lockShards

	// Start lock garbage collection go routine
	go func() {
		for {
			doGc := time.NewTimer(gcInterval)
			select {
			case <-m.stopCh:
				slog.Warn("Garbage collection exiting")
				doGc.Stop()
				close(m.stopCh)
				return
			case <-doGc.C:
				m.lockGc(gcMinIdle)
			}
		}
	}()
	return m, m.shutdown
}

// Returns the correct map shard for the provided lock name
func (m *Manager) getShard(name string) *lockShard {
	h := fnv.New32()
	if _, err := h.Write([]byte(name)); err != nil {
		panic("Error writing to fnv hash: " + err.Error())
	}
	return m.shards[h.Sum32()%uint32(len(m.shards))]
}

// shutdown stops the lock garbage collector and clears all locks
func (m *Manager) shutdown() {
	m.shutdownMtx.Lock()
	defer m.shutdownMtx.Unlock()

	slog.Warn("Stopping lock garbage collector")
	m.stopCh <- struct{}{}
	<-m.stopCh

	// Cancel the context for locks
	m.cancelCauseFn(ErrManagerShutdown)

	// Clear all unlocked locks
	m.lockGc(0 * time.Second)

	m.isShutdown = true
}

// getLock gets or creates a lock with the given name
func (m *Manager) getLock(name string, create bool, size int32) (*ManagedLock, error) {
	if size <= 0 {
		return nil, ErrInvalidLockSize
	}

	shard := m.getShard(name)
	if create {
		shard.Lock()
		defer shard.Unlock()
	} else {
		shard.RLock()
		defer shard.RUnlock()
	}

	l, ok := shard.locks[name]
	if ok {
		if create && size != l.Size() {
			// Different size requested
			return nil, ErrLockSizeMismatch
		}

		// Existing lock found
		l.lastAccessed = time.Now()
		return l, nil

	} else if !create {
		// Caller did not want a lock created
		return nil, ErrLockDoesNotExist
	}

	shard.locks[name] = NewManagedLock(name, m.ctx, size)
	return shard.locks[name], nil
}

// Lock obtains a lock on the named lock. It blocks until a lock is obtained or is canceled or
// timed out by context
func (m *Manager) Lock(name string, key string, size int32, ctx context.Context) (<-chan interface{}, error) {
	m.shutdownMtx.RLock()
	defer m.shutdownMtx.RUnlock()

	if m.isShutdown {
		return nil, ErrManagerShutdown
	}

	l, err := m.getLock(name, true, size)
	if err != nil {
		return nil, err
	}
	if l.deleted {
		panic(fmt.Sprintf("Tried to lock deleted lock %s", name))
	}
	return l.Lock.Lock(key, ctx), nil
}

// TryLock tries lock the named lock and immediately fails / succeeds
func (m *Manager) TryLock(name string, key string, size int32) (bool, error) {
	m.shutdownMtx.RLock()
	defer m.shutdownMtx.RUnlock()

	if m.isShutdown {
		return false, ErrManagerShutdown
	}

	l, err := m.getLock(name, true, size)
	if err != nil {
		return false, err
	}
	if l.deleted {
		panic(fmt.Sprintf("Tried to lock deleted lock %s", name))
	}
	return l.TryLock(key)
}

// Unlock surprisingly unlocks the named lock
func (m *Manager) Unlock(name string, key string) (bool, error) {
	m.shutdownMtx.RLock()
	defer m.shutdownMtx.RUnlock()

	if m.isShutdown {
		return false, ErrManagerShutdown
	}

	l, err := m.getLock(name, false, 1)
	if err != nil {
		return false, err
	}
	if l == nil {
		return false, ErrLockDoesNotExist
	}
	l.lockKey()
	if l.deleted {
		return false, ErrLockDoesNotExist
	}
	l.unlockKey()

	return l.Lock.Unlock(key)
}

// lockGc removes unused locks - unlocked and not accessed since <minIdle>
func (m *Manager) lockGc(minIdle time.Duration) {

	for _, shard := range m.shards {
		shard.Lock()

		slog.Debug("Starting lock garbage collection")
		numDeleted := 0
		for _, v := range shard.locks {
			v.lockKey()
			if len(v.keys) == 0 && time.Since(v.lastAccessed) > minIdle {
				v.deleted = true
				close(v.mtx)
				delete(shard.locks, v.Name)
				numDeleted++
			}
			v.unlockKey()
		}
		slog.Info(fmt.Sprintf("Lock garbage collection cleared %d locks", numDeleted))

		shard.Unlock()

	}
}

// Locks returns a list of all locks
func (m *Manager) Locks() []LockInfo {

	locks := []LockInfo{}
	for _, shard := range m.shards {
		func() {
			shard.RLock()
			defer shard.RUnlock()
			shardLocks := make([]LockInfo, len(shard.locks))
			idx := 0
			for name, l := range shard.locks {
				keys := l.Keys()
				shardLocks[idx] = LockInfo{
					Name:         name,
					Keys:         strings.Join(keys, ", "),
					LastAccessed: l.lastAccessed,
					Locked:       len(keys) > 0,
				}
				idx++
			}
			locks = append(locks, shardLocks...)
		}()
	}
	return locks
}
