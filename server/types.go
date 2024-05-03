// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
This file contains common types and interfaces for the server package
*/

package server

import (
	"context"
	"time"

	cl "github.com/imoore76/go-ldlm/server/clientlock"
	"github.com/imoore76/go-ldlm/server/ipc"
	"github.com/imoore76/go-ldlm/server/session"
)

// For context value key
type ctxKeyType struct{}

var sessionCtxKey = ctxKeyType{}

type LockServerConfig struct {
	LockGcInterval      time.Duration `desc:"Interval at which to garbage collect unused locks." default:"30m" short:"g"`
	LockGcMinIdle       time.Duration `desc:"Minimum time a lock has to be idle (no unlocks or locks) before being considered for garbage collection" default:"5m" short:"m"`
	DefaultLockTimeout  time.Duration `desc:"Lock timeout to use when loading locks from state file on startup" default:"10m" short:"d"`
	NoClearOnDisconnect bool          `desc:"Do not clear locks on client disconnect" default:"false" short:"n"`
	ipc.IPCConfig
	session.SessionConfig
}

type lockManager interface {
	Lock(name string, key string, ctx context.Context) (chan interface{}, error)
	TryLock(name string, key string) (bool, error)
	Unlock(name string, key string) (bool, error)
}

type sessionManager interface {
	CreateSession(string)
	DestroySession(string) []cl.Lock
	AddLock(name string, key string, sessionId string)
	RemoveLock(name string, sessionId string)
	Load() (map[string][]cl.Lock, error)
	Locks() map[string][]cl.Lock
}

type timerManager interface {
	Add(key string, onTimeout func(), d time.Duration)
	Remove(key string)
	Refresh(key string, d time.Duration) (bool, error)
}

// Type returned by server's locking functions
type Lock struct {
	Name   string
	Key    string
	Locked bool
}
