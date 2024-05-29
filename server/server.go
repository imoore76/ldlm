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
This file contains the ldlm lock server. It exposes functions, not ports. For networking see the
net package.
*/
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/imoore76/go-ldlm/lock"
	"github.com/imoore76/go-ldlm/log"
	cl "github.com/imoore76/go-ldlm/server/clientlock"
	"github.com/imoore76/go-ldlm/server/ipc"
	"github.com/imoore76/go-ldlm/server/session"
	"github.com/imoore76/go-ldlm/timer"
)

var (
	ErrEmptyName                    = errors.New("lock name cannot be empty")
	ErrLockWaitTimeout              = errors.New("timeout waiting to acquire lock")
	ErrLockDoesNotExistOrInvalidKey = errors.New("lock does not exist or invalid key")
	ErrSessionDoesNotExist          = errors.New("session does not exist")
	ErrInvalidLockTimeout           = errors.New("lock timeout must be greater than 0")
	ErrInvalidWaitTimeout           = errors.New("wait timeout must be greater than 0")
)

// Lock server
type LockServer struct {
	lockMgr             lockManager
	sessMgr             sessionManager
	lockTimerMgr        timerManager
	isShutdown          atomic.Bool
	noClearOnDisconnect bool
}

// New creates a new lock server.
//
// It initializes a lock manager, session manager, and lock timer manager.
// It loads locks from the session manager and tries to lock all loaded locks.
// It starts the IPC server and returns the lock server instance and a closer function.
//
// Parameters:
//   - c: a pointer to a LockServerConfig struct containing configuration options for the lock server.
//
// Returns:
//   - a pointer to a LockServer struct representing the lock server.
//   - a function that closes the lock server.
//   - an error if there was an issue creating the lock server.
func New(c *LockServerConfig) (*LockServer, func(), error) {
	lm, lmCloser := lock.NewManager(c.LockGcInterval, c.LockGcMinIdle)
	l := &LockServer{
		isShutdown:          atomic.Bool{},
		lockMgr:             lm,
		sessMgr:             session.NewManager(&c.SessionConfig),
		noClearOnDisconnect: c.NoClearOnDisconnect,
	}

	// Set up lock timer manager
	timeMgr, tmCloser := timer.NewManager()
	l.lockTimerMgr = timeMgr

	// Load locks from session manager
	sessionLocks, err := l.sessMgr.Load()
	if err != nil {
		return nil, func() {
			l.isShutdown.Store(true)
			tmCloser()
			lmCloser()
		}, err
	}

	// Try to lock all loaded locks
	ctx := context.Background()
	for sessionId, locks := range sessionLocks {
		for _, lk := range locks {

			locked, err := l.lockMgr.TryLock(lk.Name(), lk.Key())
			if err != nil || !locked {
				slog.Error("Error locking loaded locks lockMgr.TryLock()",
					"name", lk.Name(), "key", lk.Key(), "error", err.Error(),
				)
				// Locking failed, remove the loaded lock from sessMgr
				l.sessMgr.RemoveLock(lk.Name(), sessionId)
				continue
			}
			l.lockTimerMgr.Add(
				lk.Name()+lk.Key(),
				l.onTimeoutFunc(ctx, lk.Name(), lk.Key(), sessionId),
				c.DefaultLockTimeout,
			)
			slog.Info("Loaded lock", "name", lk.Name(), "key", lk.Key())
		}
	}

	// Start the IPC server
	ipcCloser, err := ipc.Run(l, &c.IPCConfig)
	if err != nil {
		return nil, func() {
			l.isShutdown.Store(true)
			tmCloser()
			lmCloser()
		}, fmt.Errorf("ipc.Run(): %w", err)
	}

	// Return instance and closer function
	return l, func() {
		ipcCloser()
		l.isShutdown.Store(true)
		tmCloser()
		lmCloser()
	}, nil
}

// Lock blocks until the lock is obtained or is canceled / timed out by context
func (l *LockServer) Lock(ctx context.Context, name string, lockTimeoutSeconds *int32, waitTimeoutSeconds *int32) (*Lock, error) {
	sessionId, ok := l.SessionId(ctx)
	if !ok {
		return nil, ErrSessionDoesNotExist
	}

	if lockTimeoutSeconds != nil && *lockTimeoutSeconds < 0 {
		return nil, ErrInvalidLockTimeout
	}
	if waitTimeoutSeconds != nil && *waitTimeoutSeconds < 0 {
		return nil, ErrInvalidWaitTimeout
	}

	key := sessionId
	ctxLog := log.FromContextOrDefault(ctx)

	lockCtx := ctx
	// Set up context with timeout if wait timeout is set
	if waitTimeoutSeconds != nil && *waitTimeoutSeconds > 0 {
		newCtx, cancel := context.WithTimeoutCause(
			ctx,
			time.Duration(*waitTimeoutSeconds)*time.Second,
			ErrLockWaitTimeout,
		)
		lockCtx = newCtx
		defer cancel()
	}

	ctxLog.Info(
		"Lock request",
		"lock", name,
	)

	var (
		locker chan interface{}
		err    error
	)

	if name == "" {
		err = ErrEmptyName
	} else {
		locker, err = l.lockMgr.Lock(name, key, lockCtx)
	}
	// Empty lock name or error from lockMgr.Lock
	if err != nil {
		ctxLog.Info(
			"Lock response",
			"lock", name,
			"locked", false,
			"error", err,
		)
		return &Lock{
			Name:   name,
			Locked: false,
		}, err
	}

	// Wait for lock response
	lr := <-locker

	var (
		lrErr  error = nil
		locked       = false
	)

	// Error from lockMgr.Lock channel
	if err, ok := lr.(error); ok {
		lrErr = err
	} else {
		// Lock acquired if an error wasn't returned
		locked = true

		// Add lock to sessMgr
		l.sessMgr.AddLock(name, key, sessionId)

		// Add lock timer if lock timeout is set
		if lockTimeoutSeconds != nil {
			d := time.Duration(*lockTimeoutSeconds) * time.Second
			l.lockTimerMgr.Add(name+key, l.onTimeoutFunc(ctx, name, key, sessionId), d)
		}
	}

	ctxLog.Info(
		"Lock response",
		"lock", name,
		"key", key,
		"locked", locked,
		"error", lrErr,
	)

	return &Lock{
		Name:   name,
		Locked: locked,
		Key:    key,
	}, lrErr
}

// Unlock surprisingly, unlocks a lock...
func (l *LockServer) Unlock(ctx context.Context, name string, key string) (bool, error) {
	sessionId, ok := l.SessionId(ctx)
	if !ok {
		return false, ErrSessionDoesNotExist
	}

	ctxLog := log.FromContextOrDefault(ctx)
	ctxLog.Info(
		"Unlock request",
		"lock", name,
		"key", key,
	)

	unlocked, err := l.lockMgr.Unlock(name, key)

	if unlocked {
		// Remove from lock map and lock timer
		l.sessMgr.RemoveLock(name, sessionId)
		l.lockTimerMgr.Remove(name + key)
	}

	ctxLog.Info(
		"Unlock response",
		"key", key,
		"lock", name,
		"unlocked", unlocked,
		"error", err,
	)

	return unlocked, err
}

// TryLock attempts to acquire the lock and immediately fails or succeeds
func (l *LockServer) TryLock(ctx context.Context, name string, lockTimeoutSeconds *int32) (*Lock, error) {
	sessionId, ok := l.SessionId(ctx)
	if !ok {
		return nil, ErrSessionDoesNotExist
	}

	if lockTimeoutSeconds != nil && *lockTimeoutSeconds < 0 {
		return nil, ErrInvalidLockTimeout
	}

	key := sessionId
	ctxLog := log.FromContextOrDefault(ctx)

	ctxLog.Info(
		"TryLock request",
		"lock", name,
	)

	var (
		locked bool
		err    error
	)

	if name == "" {
		locked = false
		err = ErrEmptyName
	} else {
		locked, err = l.lockMgr.TryLock(name, key)
	}

	if locked {
		// Add lock to sessMgr
		l.sessMgr.AddLock(name, key, sessionId)

		// Add lock timer if lock timeout is set
		if lockTimeoutSeconds != nil && *lockTimeoutSeconds > 0 {
			d := time.Duration(*lockTimeoutSeconds) * time.Second
			l.lockTimerMgr.Add(name+key, l.onTimeoutFunc(ctx, name, key, sessionId), d)
		}
	}

	ctxLog.Info(
		"TryLock response",
		"lock", name,
		"key", key,
		"locked", locked,
		"error", err,
	)

	return &Lock{
		Name:   name,
		Locked: locked,
		Key:    key,
	}, err

}

// RefreshLock refreshes a lock timer
func (l *LockServer) RefreshLock(ctx context.Context, name string, key string, lockTimeoutSeconds int32) (*Lock, error) {

	if lockTimeoutSeconds <= 0 {
		return nil, ErrInvalidLockTimeout
	}

	ctxLog := log.FromContextOrDefault(ctx)
	ctxLog.Info(
		"RefreshLock request",
		"lock", name,
		"key", key,
		"timeout", lockTimeoutSeconds,
	)

	locked, err := l.lockTimerMgr.Refresh(name+key, time.Duration(lockTimeoutSeconds)*time.Second)

	if err == timer.ErrTimerDoesNotExist {
		err = ErrLockDoesNotExistOrInvalidKey
	}

	ctxLog.Info(
		"RefreshLock response",
		"lock", name,
		"locked", locked,
		"error", err,
	)

	return &Lock{
		Name:   name,
		Locked: locked,
		Key:    key,
	}, err

}

// Locks returns a list of client locks.
func (l *LockServer) Locks() []cl.Lock {
	locks := []cl.Lock{}
	for _, ll := range l.sessMgr.Locks() {
		locks = append(locks, ll...)
	}
	return locks
}

// SessionId returns the session ID from the context.
func (l *LockServer) SessionId(ctx context.Context) (string, bool) {
	v := ctx.Value(sessionCtxKey)
	if v == nil {
		return "", false
	}
	return v.(string), true
}

// CreateSession creates a new session
func (l *LockServer) CreateSession(ctx context.Context, sessionInfo map[string]any) (string, context.Context) {
	sessionId := uuid.NewString()
	cxLogger := slog.Default().With(
		"session_id", sessionId,
	)
	for k, v := range sessionInfo {
		cxLogger = cxLogger.With(k, v)
	}
	cxLogger.Info("Session started")

	ctx = context.WithValue(ctx, sessionCtxKey, sessionId)
	ctx = log.ToContext(cxLogger, ctx)
	l.sessMgr.CreateSession(sessionId)

	return sessionId, ctx
}

// DestroySession destroys the session
func (l *LockServer) DestroySession(ctx context.Context) (sessionId string) {
	sessionId = ctx.Value(sessionCtxKey).(string)

	// This section is hit when the server is being shut down. We don't to clear all locks on
	// shutdown, otherwise the session manager will write all 0 locks to the state file on
	// shutdown.
	if l.isShutdown.Load() {
		return
	}

	ctxLog := log.FromContextOrDefault(ctx)
	ctxLog.Info("Session ended")

	locks := l.sessMgr.DestroySession(sessionId)

	if l.noClearOnDisconnect || len(locks) == 0 {
		return
	}

	ctxLog.Info("Client session cleanup",
		"num_locks", len(locks),
	)

	// For each lock, unlock it and remove it from the lockTimerMgr
	for _, lk := range locks {
		if unlocked, err := l.lockMgr.Unlock(lk.Name(), lk.Key()); err != nil || !unlocked {
			ctxLog.Error(
				"Error unlocking lock during client session cleanup",
				"lock", lk.Name(),
				"key", lk.Key(),
				"error", err,
			)

		} else {
			ctxLog.Info(
				"Unlocked during client session cleanup",
				"lock", lk.Name(),
			)
			l.lockTimerMgr.Remove(lk.Name())
		}
	}

	return
}

// onTimeoutFunc returns a function for handling a lock timeout
func (l *LockServer) onTimeoutFunc(ctx context.Context, name string, key string, sessionId string) func() {
	return func() {
		// Unlock on timeout. The lock manager will automatically remove the timer
		// when the timer function fires, so there is no need to do that here.
		ctxLog := log.FromContextOrDefault(ctx)
		ctxLog.Info(
			"Lock timer timeout",
			"lock", name,
			"key", key,
		)
		if unlocked, err := l.lockMgr.Unlock(name, key); err != nil || !unlocked {
			ctxLog.Error("Error unlocking lock during lock timer timeout",
				"lock", name,
				"key", key,
				"unlocked", unlocked,
				"error", err,
			)
		}
		// There is a race condition where a timer may fire on client disconnect and the disconnect
		// handler has already removed the lock from the session manager
		defer func() {
			if r := recover(); r != nil {
				ctxLog.Error("Error removing lock from session",
					"session_id", sessionId,
					"lock", name,
					"error", r.(string),
				)
			}
		}()
		l.sessMgr.RemoveLock(name, sessionId)
	}
}
