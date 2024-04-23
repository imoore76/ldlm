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
This file contains LockMap struct definition and methods. A LockMap holds a
mapping of client -> locks so that locks can be unlocked when a client
disconnects
*/
package lockmap

import (
	"fmt"
	log "log/slog"
	"sync"

	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
	rw "github.com/imoore76/go-ldlm/server/locksrv/lockmap/readwriter"
)

// Config defines the configuration for a LockMap instance
type LockMapConfig struct {
	rw.Config
}

// ReadWriter defines the readWriter interface
type ReadWriter interface {
	Write(map[string][]cl.ClientLock) error
	Read() (map[string][]cl.ClientLock, error)
}

// LockMap defines the server's clientLocks map
type LockMap struct {
	clientLocks    map[string][]cl.ClientLock
	clientLocksMtx sync.RWMutex
	state          ReadWriter
}

// New creates a new LockMap instance.
//
// Parameter(s): c *Config
// Returns: *LockMap
func New(c *LockMapConfig) *LockMap {
	rwr, err := rw.New(&c.Config)
	if err != nil {
		panic(err)
	}
	return &LockMap{
		clientLocks:    map[string][]cl.ClientLock{},
		clientLocksMtx: sync.RWMutex{},
		state:          rwr,
	}
}

// Locks returns the server's clientLocks map
func (l *LockMap) Locks() map[string][]cl.ClientLock {
	l.clientLocksMtx.RLock()
	defer l.clientLocksMtx.RUnlock()
	return l.clientLocks
}

// SetReadWriter sets the lockmap readwriter
func (l *LockMap) SetReadWriter(rw ReadWriter) {
	l.clientLocksMtx.Lock()
	defer l.clientLocksMtx.Unlock()
	l.state = rw
}

// Load loads the server's clientLocks map from the ReadWriter's state file
func (l *LockMap) Load() ([]cl.ClientLock, error) {
	clLocks, err := l.state.Read()
	if err != nil {
		return nil, fmt.Errorf("loadState() error reading state file: %w", err)
	} else if clLocks == nil {
		return []cl.ClientLock{}, nil
	}
	l.clientLocks = clLocks

	lcount := 0
	rLocks := make([]cl.ClientLock, 0)
	for _, locks := range l.clientLocks {
		lcount += len(locks)
		rLocks = append(rLocks, locks...)
	}
	log.Info(fmt.Sprintf("loadState() loaded %d client locks from state file", lcount))
	return rLocks, nil
}

// Save saves the server's clientLocks map to the readWriter's state file
func (l *LockMap) Save() error {
	return l.state.Write(l.clientLocks)
}

// Remove removes a lock from the client <-> lock map
func (l *LockMap) Remove(name string, sessionKey string) {
	l.clientLocksMtx.Lock()
	defer l.clientLocksMtx.Unlock()

	locks, ok := l.clientLocks[sessionKey]
	if !ok {
		panic(fmt.Sprintf("Client with session key '%s' has no clientLocks entry", sessionKey))
	}
	if locks == nil {
		panic(fmt.Sprintf("Client with session key '%s' has a nil clientLocks entry", sessionKey))
	}
	if len(locks) == 0 {
		return
	}

	newSlice := make([]cl.ClientLock, 0, len(locks)-1)
	for _, l := range locks {
		if l.Name() == name {
			continue
		}
		newSlice = append(newSlice, l)
	}
	l.clientLocks[sessionKey] = newSlice

	if err := l.Save(); err != nil {
		panic(err)
	}
}

// Add adds lock to the client -> lock map
func (l *LockMap) Add(name string, key string) {
	l.clientLocksMtx.Lock()
	defer l.clientLocksMtx.Unlock()

	l.clientLocks[key] = append(l.clientLocks[key], *cl.New(name, key))

	if err := l.Save(); err != nil {
		panic(err)
	}
}

// AddSession adds a session to the clientLocks map
func (l *LockMap) AddSession(sessionKey string) {
	l.clientLocksMtx.Lock()
	defer l.clientLocksMtx.Unlock()

	if l.clientLocks[sessionKey] == nil {
		l.clientLocks[sessionKey] = []cl.ClientLock{}
	}

}

// RemoveSession deletes the session and returns all locks associated with it
func (l *LockMap) RemoveSession(sessionKey string) []cl.ClientLock {
	l.clientLocksMtx.Lock()
	defer l.clientLocksMtx.Unlock()

	if locks, ok := l.clientLocks[sessionKey]; ok {
		delete(l.clientLocks, sessionKey)
		l.Save()
		return locks
	} else {
		return []cl.ClientLock{}
	}
}
