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

package grpc

import (
	"context"
	"time"

	"github.com/imoore76/go-ldlm/server"
)

// The struct required to configure the server. See the config package
type GrpcConfig struct {
	KeepaliveInterval time.Duration `desc:"Interval at which to send keepalive pings to client" default:"60s" short:"k"`
	KeepaliveTimeout  time.Duration `desc:"Wait this duration for the ping ack before assuming the connection is dead" default:"10s" short:"t"`
	ListenAddress     string        `desc:"Address (host:port) at which to listen" default:"localhost:3144" short:"l"`
}

// The interface required by the gRPC server to interact with a lock server
type lockServer interface {
	Lock(ctx context.Context, name string, size *int32, lockTimeoutSeconds *int32, waitTimeoutSeconds *int32) (*server.Lock, error)
	TryLock(ctx context.Context, name string, size *int32, lockTimeoutSeconds *int32) (*server.Lock, error)
	Unlock(ctx context.Context, name string, key string) (bool, error)
	RefreshLock(ctx context.Context, name string, key string, lockTimeoutSeconds int32) (*server.Lock, error)
	DestroySession(ctx context.Context) string
	CreateSession(ctx context.Context, metadata map[string]any) (string, context.Context)
}
