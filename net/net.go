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
Package net contains the network configuration and a single Run() function which starts the
configured server(s).
*/
package net

import (
	"github.com/imoore76/go-ldlm/net/grpc"
	"github.com/imoore76/go-ldlm/net/rest"
	sec "github.com/imoore76/go-ldlm/net/security"
	"github.com/imoore76/go-ldlm/server"
)

type NetConfig struct {
	grpc.GrpcConfig
	rest.RestConfig
	sec.SecurityConfig
}

// Run runs the network server with the given lock server and network configuration.
//
// It creates a new service using the lock server, and then runs the gRPC server
// with the provided gRPC and security configurations. If the rest listen address
// is not empty, it also runs the REST server with the provided REST and security
// configurations.
//
// Parameters:
// - ls: A pointer to the lock server.
// - conf: A pointer to the network configuration.
//
// Returns:
// - A function that can be called to close the network server.
// - An error if there was a problem running the network server.
func Run(ls *server.LockServer, conf *NetConfig) (func(), error) {
	service := grpc.NewService(ls)
	grpcClose, err := grpc.Run(service, &conf.GrpcConfig, &conf.SecurityConfig)
	if err != nil {
		return nil, err
	}
	if conf.RestListenAddress != "" {
		restClose, err := rest.Run(service, &conf.RestConfig, &conf.SecurityConfig)
		if err != nil {
			grpcClose()
			return nil, err
		}
		return func() {
			restClose()
			grpcClose()
		}, nil
	}
	return grpcClose, nil
}
