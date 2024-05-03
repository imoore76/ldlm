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
This file contains sessionManager struct definition and methods. A sessionManager holds a
mapping of client -> locks so that locks can be unlocked when a client disconnects
*/
package session

import (
	"fmt"
	"maps"
	"sync"

	cl "github.com/imoore76/go-ldlm/server/clientlock"
	"github.com/imoore76/go-ldlm/server/session/store"
)

// SessionConfig defines the configuration for a sessionManager instance
type SessionConfig struct {
	StateFile string `desc:"File in which to store lock state" default:"" short:"s"`
}

// sessionManager holds the server's sessionLocks map
type sessionManager struct {
	sessionLocks    map[string][]cl.Lock
	sessionLocksMtx sync.RWMutex
	store           sessionStorer
}

// New creates a new sessionManager instance.
//
// Parameter(s): c *SessionConfig
// Returns: *sessionManager
func NewManager(c *SessionConfig) *sessionManager {
	s, err := store.New(c.StateFile)
	if err != nil {
		panic(err)
	}
	return &sessionManager{
		sessionLocks:    map[string][]cl.Lock{},
		sessionLocksMtx: sync.RWMutex{},
		store:           s,
	}
}

// Locks returns a clone of the server's sessionLocks map
func (l *sessionManager) Locks() map[string][]cl.Lock {
	l.sessionLocksMtx.Lock()
	defer l.sessionLocksMtx.Unlock()

	return maps.Clone(l.sessionLocks)
}

// SetStore sets the session manager's store
func (l *sessionManager) SetStore(s sessionStorer) {
	l.sessionLocksMtx.Lock()
	defer l.sessionLocksMtx.Unlock()
	l.store = s
}

// Load loads the server's sessionLocks map from the store's state file
func (l *sessionManager) Load() (map[string][]cl.Lock, error) {
	clLocks, err := l.store.Read()
	if err != nil {
		return nil, fmt.Errorf("loadState() error reading state file: %w", err)
	} else if clLocks == nil {
		return map[string][]cl.Lock{}, nil
	}
	l.sessionLocks = clLocks

	return l.sessionLocks, nil
}

// Save saves the server's sessionLocks map to the readWriter's state file
func (l *sessionManager) Save() error {
	return l.store.Write(l.sessionLocks)
}

// Remove removes a lock from the client <-> lock map
func (l *sessionManager) RemoveLock(name string, sessionId string) {
	l.sessionLocksMtx.Lock()
	defer l.sessionLocksMtx.Unlock()

	locks, ok := l.sessionLocks[sessionId]
	if !ok {
		panic(fmt.Sprintf("Client with session id '%s' has no session entry", sessionId))
	}
	if locks == nil {
		panic(fmt.Sprintf("Client with session id '%s' has a nil session entry", sessionId))
	}
	if len(locks) == 0 {
		return
	}

	newSlice := make([]cl.Lock, 0, len(locks)-1)
	for _, l := range locks {
		if l.Name() == name {
			continue
		}
		newSlice = append(newSlice, l)
	}
	l.sessionLocks[sessionId] = newSlice

	if err := l.Save(); err != nil {
		panic(err)
	}
}

// Add adds lock to the client -> lock map
func (l *sessionManager) AddLock(name string, key string, sessionId string) {
	l.sessionLocksMtx.Lock()
	defer l.sessionLocksMtx.Unlock()

	l.sessionLocks[sessionId] = append(l.sessionLocks[sessionId], cl.New(name, key))

	if err := l.Save(); err != nil {
		panic(err)
	}
}

// CreateSession adds a session to the sessionLocks map
func (l *sessionManager) CreateSession(sessionId string) {
	l.sessionLocksMtx.Lock()
	defer l.sessionLocksMtx.Unlock()

	if l.sessionLocks[sessionId] == nil {
		l.sessionLocks[sessionId] = []cl.Lock{}
	}
}

// DestroySession deletes the session and returns all locks associated with it
func (l *sessionManager) DestroySession(sessionId string) []cl.Lock {
	l.sessionLocksMtx.Lock()
	defer l.sessionLocksMtx.Unlock()

	if locks, ok := l.sessionLocks[sessionId]; ok {
		delete(l.sessionLocks, sessionId)
		l.Save()
		return locks
	} else {
		return []cl.Lock{}
	}
}
