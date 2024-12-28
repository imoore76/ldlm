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

package rest

import (
	"context"
	"sync"
	"time"

	pb "github.com/imoore76/ldlm/protos"
	"google.golang.org/grpc/stats"
)

type timerManager interface {
	Add(key string, onTimeout func(), d time.Duration)
	Remove(key string)
	Renew(key string, d time.Duration) (bool, error)
}

type grpcLockServer interface {
	stats.Handler
	pb.LDLMServer
}

type RestConfig struct {
	RestListenAddress  string        `desc:"Address (host:port) at which the REST server should listen. Set to '' to disable" default:"" short:"r"`
	RestSessionTimeout time.Duration `desc:"REST session idle timeout" default:"10m"`
}

type session struct {
	ctx context.Context
	mtx sync.Mutex
}
