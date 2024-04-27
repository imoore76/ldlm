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
This file contains the IPC server implementation. IPC methods are defined in
ipc.go
*/
package ipc

import (
	"context"
	"errors"
	"fmt"
	log "log/slog"
	"net"
	"net/http"
	"net/rpc"
	"os"
	fp "path/filepath"
	"runtime"
	"sync"

	pb "github.com/imoore76/go-ldlm/protos"
	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
)

// errSocketExists is the error returned when a socket file already exists.
var errSocketExists = errors.New(
	"IPC socket file already exists. If another instance of " +
		"ldlm isn't running, delete the socket file and try again. " +
		"Otherwise, shut down that instance before starting a new " +
		"one. Alternatively specify a different IPC socket file",
)

// LockServer is an interface for interacting with locks. It must support
// methods required by IPC commands
type LockServer interface {
	Locks() []cl.ClientLock
	Unlock(context.Context, *pb.UnlockRequest) (*pb.UnlockResponse, error)
}

// Config is the configuration for the IPC server.
type IPCConfig struct {
	IPCSocketFile string `desc:"Path to the IPC socket file. Use an empty string to disable IPC." default:"<platform specific>"`
}

// IPC is the IPC server implementation.
type IPC struct {
	lckSrv LockServer
}

// Create and register the singleton IPC server with net/rpc. Registering the
// same server twice would result in a panic.
var (
	singletonIpc    *IPC
	setSingletonIpc = sync.OnceFunc(func() {
		singletonIpc = new(IPC)
		if err := rpc.Register(singletonIpc); err != nil {
			panic(fmt.Errorf("rpc.Register() error: %w", err))
		}
		rpc.HandleHTTP()
	})
)

// setUp gets or creates the singleton IPC server and returns a cleanup func
func setUp(l LockServer) func() {
	setSingletonIpc()
	singletonIpc.lckSrv = l

	return func() {
		// Delete ref to lock server
		singletonIpc.lckSrv = nil
	}
}

// Run starts the IPC server and returns a cleanup function and an error.
//
// It takes a LockServer and an IPCConfig pointer as parameters.
// It returns a cleanup function and an error.
func Run(lsrv LockServer, c *IPCConfig) (func(), error) {

	if c.IPCSocketFile == "<platform specific>" {
		c.IPCSocketFile = DefaultSocketPath()
	} else if c.IPCSocketFile == "" {
		log.Warn("IPC server disabled because of empty socket file path")
		return func() {}, nil
	}

	if socketPathExists(c.IPCSocketFile) {
		return func() {}, fmt.Errorf(
			"file: %s exists. %w", c.IPCSocketFile, errSocketExists)
	}

	// Set up the IPC server
	closer := setUp(lsrv)

	l, err := net.Listen("unix", c.IPCSocketFile)
	if err != nil {
		closer()
		return func() {}, fmt.Errorf("ipc.Run() net.listen error: %w", err)
	}
	go http.Serve(l, nil)

	log.Info("IPC server started", "socket", c.IPCSocketFile)

	// Return a cleanup function
	return func() {
		log.Info("IPC server shutting down...")
		// Close listener
		if err := l.Close(); err != nil {
			log.Error("error closing listener: %v", err)
		}

		// IPC server cleanup
		closer()

		// Remove socket file
		if socketPathExists(c.IPCSocketFile) {
			if err := os.Remove(c.IPCSocketFile); err != nil {
				panic(fmt.Sprintf("error removing socket file: %v", err))
			}
		}
	}, nil
}

// DefaultSocketPath returns a default socket path based on the os
func DefaultSocketPath() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\ldlm-ipc`
	}
	return fp.Join(os.TempDir(), "ldlm-ipc.sock")
}

// socketPathExists checks if socket file path exists
func socketPathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
