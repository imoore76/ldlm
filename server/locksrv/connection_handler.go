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
This file contains LockSrv methods for handling client connects and
disconnects. It is specified as a grpc.StatsHandler and implements the
grpc.stats.Handler interface. This is the only way to hook into client
connect and disconnect events in a gRPC server in go.
*/
package locksrv

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/stats"

	"github.com/imoore76/go-ldlm/log"
)

// TagConn tags each new connection with a unique session key that is used when
// the client associated with this connection locks any lock.
func (l *LockSrv) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	sessionKey := uuid.NewString()

	cxLogger := slog.Default().With(
		"session_key", sessionKey,
		"remote_address", info.RemoteAddr.String(),
	)
	cxLogger.Info("Client connected")

	ctx = context.WithValue(ctx, sessionCtxKey, sessionKey)
	ctx = log.ToContext(cxLogger, ctx)
	return ctx
}

// HandleConn handles connection and disconnection of clients
func (l *LockSrv) HandleConn(ctx context.Context, s stats.ConnStats) {
	switch s.(type) {

	// Client connect
	case *stats.ConnBegin:
		sessionKey := ctx.Value(sessionCtxKey).(string)
		l.lockMap.AddSession(sessionKey)

	// Client disconnect
	case *stats.ConnEnd:

		// This section is hit when the server is being shut down. We don't
		// to clear all locks on shutdown, otherwise the lockMap readwriter
		// will write all 0 locks to the state file on shutdown.
		if l.isShutdown {
			return
		}

		sessionKey := ctx.Value(sessionCtxKey).(string)
		ctxLog := log.FromContextOrDefault(ctx)

		if !l.noClearOnDisconnect {
			locks := l.lockMap.RemoveSession(sessionKey)

			if len(locks) == 0 {
				return
			}

			// For each lock, unlock it and remove it from the lockTimerMgr
			for _, lk := range locks {
				if unlocked, err := l.lockMgr.Unlock(lk.Name(), lk.Key()); err != nil || !unlocked {
					ctxLog.Error(
						"Error unlocking lock during client disconnect cleanup",
						"lock", lk.Name(),
						"key", lk.Key(),
						"error", err,
					)

				} else {
					ctxLog.Info(
						"Unlocked during client disconnect cleanup",
						"lock", lk.Name(),
					)
					l.lockTimerMgr.Remove(lk.Name(), lk.Key())
				}
			}

		}
	}
}

// Unimplemented methods, but needed to satisfy grpc.stats.Handler interface
func (l *LockSrv) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context { return ctx }

func (l *LockSrv) HandleRPC(ctx context.Context, s stats.RPCStats) {}
