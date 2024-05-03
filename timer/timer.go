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
This file contains the timer manager struct and methods. A timer manager is used for handling a
map of timers which perform a callback when they expire. They can be removed or refreshed before
they expire.
*/
package timer

import (
	"errors"
	"sync"
	"time"
)

var ErrTimerDoesNotExist = errors.New("timer does not exist")

// manager for a map of timers
type manager struct {
	timers    map[string]*time.Timer
	timersMtx sync.RWMutex
}

// NewManager initializes a new timer manager with the provided onTimeoutFunc.
//
// Returns a pointer to Manager and a shutdown function.
func NewManager() (*manager, func()) {

	m := &manager{
		timers:    make(map[string]*time.Timer),
		timersMtx: sync.RWMutex{},
	}
	return m, m.shutdown
}

// Add creates and adds a timer to the map
func (m *manager) Add(key string, onTimeout func(), timeout time.Duration) {
	m.timersMtx.Lock()
	defer m.timersMtx.Unlock()

	m.timers[key] = time.AfterFunc(
		timeout,
		func() {
			onTimeout()
			m.Remove(key)
		},
	)
}

// Remove removes a timer from the map
func (m *manager) Remove(key string) {
	m.timersMtx.Lock()
	defer m.timersMtx.Unlock()

	if _, ok := m.timers[key]; ok {
		m.timers[key].Stop()
		delete(m.timers, key)
	}

}

// Refresh refreshes a timer
func (m *manager) Refresh(key string, timeout time.Duration) (bool, error) {
	m.timersMtx.RLock()
	t, ok := m.timers[key]
	m.timersMtx.RUnlock()

	if ok {
		if t.Stop() {
			t.Reset(timeout)
			return true, nil
		} else {
			return false, nil
		}
	}
	return false, ErrTimerDoesNotExist
}

// shutdown stops the timer manager which stops all timers
func (m *manager) shutdown() {
	m.timersMtx.Lock()
	defer m.timersMtx.Unlock()

	for _, t := range m.timers {
		t.Stop()
	}
	m.timers = map[string]*time.Timer{}
}
