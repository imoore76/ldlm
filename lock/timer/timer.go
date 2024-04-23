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
This file contains the lock timer manager struct and methods. A lock timer
manager is used for handling a map of lock timers which perform a callback
when they expire. They can be removed or refreshed before they expire.
*/
package timer

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Can be returned from Refresh() when the lock + key combo is not found in the
// lockTimers map
var ErrLockDoesNotExistOrInvalidKey = errors.New("lock does not exist or an invalid key was used")

// onTimeoutFunc function to run when lock times out
type onTimeoutFunc = func(name string, key string) (bool, error)

// Manager for a map of lock times
type Manager struct {
	lockTimers    map[string]*time.Timer
	lockTimersMtx sync.RWMutex
	onTimeoutFunc onTimeoutFunc
}

// NewManager initializes a new Manager with the provided onTimeoutFunc.
//
// Parameter: onTimeoutFunc function to run when lock times out
// Returns a pointer to Manager and a shutdown function.
func NewManager(otf onTimeoutFunc) (m *Manager, shutdown func()) {

	m = &Manager{
		lockTimers:    make(map[string]*time.Timer),
		lockTimersMtx: sync.RWMutex{},
		onTimeoutFunc: otf,
	}
	return m, m.shutdown
}

// Add creates and adds a lock timer to the map
func (m *Manager) Add(name string, key string, timeout time.Duration) {
	m.lockTimersMtx.Lock()
	defer m.lockTimersMtx.Unlock()

	m.lockTimers[name+key] = time.AfterFunc(
		timeout,
		func() {
			slog.Warn(
				"Lock timed out",
				"lock", name,
			)
			ok, err := m.onTimeoutFunc(name, key)
			if err != nil || !ok {
				slog.Error(
					"Error running onTimeoutFunc() on lock timeout",
					"lock", name,
					"key", key,
					"error", err,
				)
			}

		},
	)
}

// Remove removes a lock timer from the map
func (m *Manager) Remove(name string, key string) {
	m.lockTimersMtx.Lock()
	defer m.lockTimersMtx.Unlock()

	if _, ok := m.lockTimers[name+key]; ok {
		m.lockTimers[name+key].Stop()
		delete(m.lockTimers, name+key)
	}

}

// Refresh refreshes a lock timer
func (m *Manager) Refresh(name string, key string, timeout time.Duration) (bool, error) {
	m.lockTimersMtx.RLock()
	defer m.lockTimersMtx.RUnlock()

	if t, ok := m.lockTimers[name+key]; ok {
		if t.Stop() {
			t.Reset(timeout)
			return true, nil
		} else {
			slog.Warn(
				"Can't refresh lock. Timer already fired.",
				"lock", name,
				"key", key,
			)
			return false, nil
		}
	} else {
		return false, ErrLockDoesNotExistOrInvalidKey
	}
}

// shutdown stops the lock timer manager which stops all timers
func (m *Manager) shutdown() {
	m.lockTimersMtx.Lock()
	defer m.lockTimersMtx.Unlock()

	for _, t := range m.lockTimers {
		t.Stop()
	}
	m.lockTimers = map[string]*time.Timer{}
}
