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
map of timers which perform a callback when they expire. They can be removed or renewed before
they expire.
*/
package timermap

import (
	"errors"
	"sync"
	"time"
)

var ErrTimerDoesNotExist = errors.New("timer does not exist")

// manager for a map of timers
type TimerMap struct {
	timers    map[string]*time.Timer
	timersMtx sync.RWMutex
}

// New initializes a new timer manager with the provided onTimeoutFunc.
//
// Returns a pointer to TimerMap and a closer function.
func New() (*TimerMap, func()) {

	m := &TimerMap{
		timers:    make(map[string]*time.Timer),
		timersMtx: sync.RWMutex{},
	}
	return m, m.shutdown
}

// Add creates and adds a timer to the map
func (m *TimerMap) Add(key string, onTimeout func(), timeout time.Duration) {
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
func (m *TimerMap) Remove(key string) {
	m.timersMtx.Lock()
	defer m.timersMtx.Unlock()

	if _, ok := m.timers[key]; ok {
		m.timers[key].Stop()
		delete(m.timers, key)
	}

}

// Reset resets a timer. It returns true if the timer was reset, false if the timer
// has already fired or has been stopped. If the timer does not exist, an error is
// also returned.
func (m *TimerMap) Reset(key string, timeout time.Duration) (bool, error) {
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
func (m *TimerMap) shutdown() {
	m.timersMtx.Lock()
	defer m.timersMtx.Unlock()

	for _, t := range m.timers {
		t.Stop()
	}
	m.timers = nil
}
