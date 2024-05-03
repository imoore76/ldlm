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
	"log/slog"
	"sync/atomic"
	"time"
)

var ErrLockDoesNotExist = errors.New("a lock by that name does not exist")

// A ManagedLock is a lock with a some record keeping fields for
// garbage collection
type ManagedLock struct {
	Name         string
	lastAccessed time.Time
	deleted      bool
	*Lock
}

// LockInfo is a struct that contains only basic information about a lock
type LockInfo struct {
	Name         string
	Key          string // The lock key
	LastAccessed time.Time
	Locked       bool
}

// NewManagedLock returns a managed lock object with the given name
func NewManagedLock(name string) *ManagedLock {
	return &ManagedLock{
		Name:         name,
		lastAccessed: time.Now(),
		Lock:         NewLock(),
	}
}

// A Manager polices access to a group of locks addressable by their names
type Manager struct {
	lockCh     chan struct{}
	locks      map[string]*ManagedLock
	stopCh     chan struct{}
	isShutdown atomic.Bool
}

// NewManager creates a new Manager with the given garbage collection interval, minimum idle time, and logger, and returns a pointer to the Manager.
//
// Parameters:
// - ctx context.Context: the context for the Manager
// - gcInterval time.Duration: the interval for lock garbage collection
// - gcMinIdle time.Duration: the minimum time a lock must be idle before being considered for garbage collection
// Returns:
// - *Manager: a pointer to the newly created Manager
func NewManager(gcInterval time.Duration, gcMinIdle time.Duration) (*Manager, func()) {

	m := &Manager{
		lockCh: make(chan struct{}, 1),
		locks:  make(map[string]*ManagedLock),
		stopCh: make(chan struct{}),
	}

	// Start lock garbage collection go routine
	go func() {
		for {
			select {
			case <-m.stopCh:
				slog.Warn("Garbage collection exiting")
				close(m.stopCh)
				return
			case <-time.After(gcInterval):
				m.lockGc(gcMinIdle)
			}
		}
	}()
	return m, m.shutdown
}

// Lock access to managed locks
func (m *Manager) lock() {
	m.lockCh <- struct{}{}
}

// Unlock access to managed locks
func (m *Manager) unlock() {
	<-m.lockCh
}

// shutdown stops the lock garbage collector and clears all locks
func (m *Manager) shutdown() {

	slog.Warn("Stopping lock garbage collector")
	m.stopCh <- struct{}{}

	// Clear all locks
	m.lockGc(0 * time.Second)

	m.isShutdown.Store(true)
}

// getLock gets or creates a lock with the given name
func (m *Manager) getLock(name string, create bool) *ManagedLock {
	m.lock()
	defer m.unlock()

	if m.isShutdown.Load() {
		panic("Attempted to get lock from a stopped Manager")
	}

	l, ok := m.locks[name]
	if ok {
		// Existing lock found
		l.lastAccessed = time.Now()
		return l
	} else if !create {
		// Caller did not want a lock created
		return nil
	}

	m.locks[name] = NewManagedLock(name)
	return m.locks[name]
}

// Lock obtains a lock on the named lock. It blocks until a lock is obtained or is canceled / timed out by context
func (m *Manager) Lock(name string, key string, ctx context.Context) (chan interface{}, error) {
	l := m.getLock(name, true)
	if l.deleted {
		panic(fmt.Sprintf("Tried to lock deleted lock %s", name))
	}
	return l.Lock.Lock(key, ctx), nil
}

// TryLock tries lock the named lock and immediately fails / succeeds
func (m *Manager) TryLock(name string, key string) (bool, error) {
	l := m.getLock(name, true)
	if l.deleted {
		panic(fmt.Sprintf("Tried to lock deleted lock %s", name))
	}
	return l.Lock.TryLock(key)
}

// Unlock surprisingly unlocks the named lock
func (m *Manager) Unlock(name string, key string) (bool, error) {
	l := m.getLock(name, false)
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
	m.lock()
	defer m.unlock()

	slog.Debug("Starting lock garbage collection")
	numDeleted := 0
	for _, v := range m.locks {
		v.lockKey()
		if v.key == "" && time.Since(v.lastAccessed) > minIdle {
			v.deleted = true
			close(v.mtx)
			delete(m.locks, v.Name)
			numDeleted++
		}
		v.unlockKey()
	}
	slog.Info(fmt.Sprintf("Lock garbage collection cleared %d locks", numDeleted))
}

// Locks returns a list of all locks
func (m *Manager) Locks() []LockInfo {
	m.lock()
	defer m.unlock()

	locks := make([]LockInfo, len(m.locks))
	idx := 0
	for name, l := range m.locks {
		l.lockKey()
		locks[idx] = LockInfo{
			Name:         name,
			Key:          l.key,
			LastAccessed: l.lastAccessed,
			Locked:       l.key != "",
		}
		l.unlockKey()
		idx++
	}
	return locks
}
