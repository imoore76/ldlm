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
This file contains the gRPC exposed, protocol buffer defined services for the
ldlm lock server. The ldlm lock server is essentially a gRPC wrapper around a
single lock.Manager which manages a group of locks.
*/
package locksrv

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/imoore76/go-ldlm/lock"
	"github.com/imoore76/go-ldlm/lock/timer"
	"github.com/imoore76/go-ldlm/log"
	pb "github.com/imoore76/go-ldlm/protos"
	"github.com/imoore76/go-ldlm/server/locksrv/ipc"
	"github.com/imoore76/go-ldlm/server/locksrv/lockmap"
	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
)

var (
	ErrLockWaitTimeout = errors.New("timeout waiting to acquire lock")
	ErrEmptyName       = errors.New("lock name cannot be empty")
)

// For context value key
type ldlmCtxKeyType struct{}

var sessionCtxKey = ldlmCtxKeyType{}

type lockManager interface {
	Lock(name string, key string, ctx context.Context) (chan interface{}, error)
	TryLock(name string, key string) (bool, error)
	Unlock(name string, key string) (bool, error)
}

type lockMapper interface {
	AddSession(string)
	RemoveSession(string) []cl.ClientLock
	Add(name string, sessionKey string)
	Remove(name string, sessionKey string)
	Load() ([]cl.ClientLock, error)
	Locks() map[string][]cl.ClientLock
}

type lockTimerManager interface {
	Add(name string, sessionKey string, d time.Duration)
	Remove(name string, sessionKey string)
	Refresh(name string, key string, d time.Duration) (bool, error)
}

type LockSrvConfig struct {
	LockGcInterval      time.Duration `desc:"Interval at which to garbage collect unused locks." default:"30m" short:"g"`
	LockGcMinIdle       time.Duration `desc:"Minimum time a lock has to be idle (no unlocks or locks) before being considered for garbage collection" default:"5m" short:"m"`
	DefaultLockTimeout  time.Duration `desc:"Lock timeout to use when loading locks from state file on startup" default:"10m" short:"d"`
	NoClearOnDisconnect bool          `desc:"Do not clear locks on client disconnect" default:"false" short:"n"`
	ipc.IPCConfig
	lockmap.LockMapConfig
}

// RPC and connection handler struct
type LockSrv struct {
	lockMgr             lockManager
	lockMap             lockMapper
	lockTimerMgr        lockTimerManager
	isShutdown          bool
	noClearOnDisconnect bool
	pb.UnsafeLDLMServer
}

// New creates a new lock server
func New(c *LockSrvConfig) (*LockSrv, func()) {
	lm, lmCloser := lock.NewManager(c.LockGcInterval, c.LockGcMinIdle)
	l := &LockSrv{
		isShutdown:          false,
		lockMgr:             lm,
		lockMap:             lockmap.New(&c.LockMapConfig),
		noClearOnDisconnect: c.NoClearOnDisconnect,
	}

	// Set up lock timer manager
	var ltmCloser func()
	l.lockTimerMgr, ltmCloser = timer.NewManager(func(name string, key string) (bool, error) {
		ok, err := l.lockMgr.Unlock(name, key)
		if err == nil && ok {
			l.lockTimerMgr.Remove(name, key)
			l.lockMap.Remove(name, key)
		}
		return ok, err
	})

	// Load locks from lockMap
	locks, err := l.lockMap.Load()
	if err != nil {
		panic("lockMap.Load() " + err.Error())
	}

	// Try to lock all loaded locks
	for _, lk := range locks {
		locked, err := l.lockMgr.TryLock(lk.Name(), lk.Key())
		if err != nil || !locked {
			slog.Error("Error locking loaded locks lockMgr.TryLock()",
				"name", lk.Name(), "key", lk.Key(), "error", err.Error(),
			)
			// Locking failed, remove the loaded lock from lockMap
			l.lockMap.Remove(lk.Name(), lk.Key())
			continue
		}
		l.lockTimerMgr.Add(lk.Name(), lk.Key(), c.DefaultLockTimeout)
		slog.Info("Loaded lock", "name", lk.Name(), "key", lk.Key())
	}

	// Start the IPC server
	ipcStopper, err := ipc.Run(l, &c.IPCConfig)
	if err != nil {
		if ipcStopper != nil {
			ipcStopper()
		}
		panic("ipc.Run() " + err.Error())
	}

	// Return instance and closer function
	return l, func() {
		ipcStopper()
		ltmCloser()
		l.isShutdown = true
		lmCloser()
	}
}

// Lock blocks until the lock is obtained or is canceled / timed out by context
func (l *LockSrv) Lock(ctx context.Context, r *pb.LockRequest) (*pb.LockResponse, error) {
	sessionKey := ctx.Value(sessionCtxKey).(string)
	ctxLog := log.FromContextOrDefault(ctx)

	lockCtx := ctx
	// Set up context with timeout if wait timeout is set
	if r.WaitTimeoutSeconds != nil {
		newCtx, cancel := context.WithTimeoutCause(
			ctx,
			time.Duration(*r.WaitTimeoutSeconds)*time.Second,
			ErrLockWaitTimeout,
		)
		lockCtx = newCtx
		defer cancel()
	}

	ctxLog.Info(
		"Lock request",
		"lock", r.Name,
	)

	var (
		locker chan interface{}
		err    error
	)

	if r.Name == "" {
		err = ErrEmptyName
	} else {
		locker, err = l.lockMgr.Lock(r.Name, sessionKey, lockCtx)
	}
	// Empty lock name or error from lockMgr.Lock
	if err != nil {
		ctxLog.Info(
			"Lock response",
			"lock", r.Name,
			"locked", false,
			"error", err,
		)
		return &pb.LockResponse{
			Name:   r.Name,
			Locked: false,
			Key:    sessionKey,
			Error:  lockErrToRPCErr(err),
		}, nil
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

		// Add lock to lockMap
		l.lockMap.Add(r.Name, sessionKey)

		// Add lock timer if lock timeout is set
		if r.LockTimeoutSeconds != nil && *r.LockTimeoutSeconds > 0 {
			d := time.Duration(*r.LockTimeoutSeconds) * time.Second
			l.lockTimerMgr.Add(r.Name, sessionKey, d)
		}

	}

	ctxLog.Info(
		"Lock response",
		"lock", r.Name,
		"locked", locked,
		"error", lrErr,
	)

	return &pb.LockResponse{
		Name:   r.Name,
		Locked: locked,
		Key:    sessionKey,
		Error:  lockErrToRPCErr(lrErr),
	}, nil
}

// Unlock removes the lock held by the client
func (l *LockSrv) Unlock(ctx context.Context, r *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	ctxLog := log.FromContextOrDefault(ctx)
	ctxLog.Info(
		"Unlock request",
		"lock", r.Name,
		"key", r.Key,
	)

	unlocked, err := l.lockMgr.Unlock(r.Name, r.Key)

	if unlocked {
		// Remove from lock map and lock timer
		l.lockMap.Remove(r.Name, r.Key)
		l.lockTimerMgr.Remove(r.Name, r.Key)
	}

	ctxLog.Info(
		"Unlock response",
		"key", r.Key,
		"lock", r.Name,
		"unlocked", unlocked,
		"error", err,
	)

	return &pb.UnlockResponse{
		Unlocked: unlocked,
		Name:     r.Name,
		Error:    lockErrToRPCErr(err),
	}, nil
}

// TryLock attempts to acquire the lock and immediately fails or succeeds
func (l *LockSrv) TryLock(ctx context.Context, r *pb.TryLockRequest) (*pb.LockResponse, error) {
	sessionKey := ctx.Value(sessionCtxKey).(string)
	ctxLog := log.FromContextOrDefault(ctx)

	ctxLog.Info(
		"Lock request",
		"lock", r.Name,
	)

	var (
		locked bool
		err    error
	)

	if r.Name == "" {
		locked = false
		err = ErrEmptyName
	} else {
		locked, err = l.lockMgr.TryLock(r.Name, sessionKey)
	}

	if locked {
		// Add lock to lockMap
		l.lockMap.Add(r.Name, sessionKey)

		// Add lock timer if lock timeout is set
		if r.LockTimeoutSeconds != nil && *r.LockTimeoutSeconds > 0 {
			d := time.Duration(*r.LockTimeoutSeconds) * time.Second
			l.lockTimerMgr.Add(r.Name, sessionKey, d)
		}
	}

	ctxLog.Info(
		"Lock response",
		"lock", r.Name,
		"locked", locked,
		"error", err,
	)

	return &pb.LockResponse{
		Name:   r.Name,
		Locked: locked,
		Key:    sessionKey,
		Error:  lockErrToRPCErr(err),
	}, nil

}

// RefreshLock refreshes a lock timer
func (l *LockSrv) RefreshLock(ctx context.Context, r *pb.RefreshLockRequest) (*pb.LockResponse, error) {
	ctxLog := log.FromContextOrDefault(ctx)
	ctxLog.Info(
		"RefreshLock request",
		"lock", r.Name,
		"key", r.Key,
		"timeout", r.LockTimeoutSeconds,
	)

	locked, err := l.lockTimerMgr.Refresh(r.Name, r.Key, time.Duration(r.LockTimeoutSeconds)*time.Second)

	ctxLog.Info(
		"RefreshLock response",
		"lock", r.Name,
		"locked", locked,
		"error", err,
	)

	return &pb.LockResponse{
		Name:   r.Name,
		Locked: locked,
		Key:    r.Key,
		Error:  lockErrToRPCErr(err),
	}, nil

}

// Locks returns a list of client locks.
func (l *LockSrv) Locks() []cl.ClientLock {
	locks := []cl.ClientLock{}
	for _, ll := range l.lockMap.Locks() {
		locks = append(locks, ll...)
	}
	return locks
}

// lockErrToRPCErr converts an error to an RPC error
func lockErrToRPCErr(e error) *pb.Error {
	if e == nil {
		return nil
	}

	var errCode pb.ErrorCode = pb.ErrorCode_Unknown
	switch e {
	case ErrLockWaitTimeout:
		errCode = pb.ErrorCode_LockWaitTimeout
	case lock.ErrInvalidLockKey:
		errCode = pb.ErrorCode_InvalidLockKey
	case lock.ErrLockDoesNotExist:
		errCode = pb.ErrorCode_LockDoesNotExist
	case lock.ErrLockNotLocked:
		errCode = pb.ErrorCode_NotLocked
	case timer.ErrLockDoesNotExistOrInvalidKey:
		errCode = pb.ErrorCode_LockDoesNotExistOrInvalidKey
	}
	return &pb.Error{
		Code:    errCode,
		Message: e.Error(),
	}

}
