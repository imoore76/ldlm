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
	"io/fs"
	log "log/slog"
	"net"
	"net/http"
	"net/rpc"
	"os"
	fp "path/filepath"
	"runtime"
	"sync"

	cl "github.com/imoore76/ldlm/server/clientlock"
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
	Locks() []cl.Lock
	Unlock(context.Context, string, string) (bool, error)
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
func setUp(l LockServer) {
	setSingletonIpc()
	singletonIpc.lckSrv = l
}

// Run runs the IPC server with the provided LockServer and IPCConfig.
//
// It sets up the IPC server, listens on the IPC socket, handles server start and shutdown,
// and performs cleanup operations.
// Parameters:
//   - ctx: the context.Context for managing the lifecycle of the IPC server.
//   - lsrv: the LockServer implementation for handling lock operations.
//   - c: the IPCConfig containing IPC server configuration.
//
// Returns:
//   - cleanup: a function that can be called to clean up the IPC server.
//   - error: an error if the IPC server failed to start.
func Run(lsrv LockServer, c *IPCConfig) (func(), error) {

	if c.IPCSocketFile == "<platform specific>" {
		c.IPCSocketFile = DefaultSocketPath()
	} else if c.IPCSocketFile == "" {
		log.Warn("IPC server disabled because of empty socket file path")
		return func() {}, nil
	}

	if socketPathExists(c.IPCSocketFile) {
		return nil, fmt.Errorf(
			"file: %s exists. %w", c.IPCSocketFile, errSocketExists)
	}

	// Set up the IPC server
	setUp(lsrv)

	l, err := net.Listen("unix", c.IPCSocketFile)
	if err != nil {
		return nil, fmt.Errorf("ipc.Run() net.listen error: %w", err)
	}
	go http.Serve(l, nil)

	log.Info("IPC server started", "socket", c.IPCSocketFile)

	// Cleanup function
	return func() {
		log.Info("IPC server shutting down...")
		// Close listener
		if err := l.Close(); err != nil {
			log.Error("error closing listener", "error", err)
		}

		log.Info("IPC server shut down", "file", c.IPCSocketFile)
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
	return !errors.Is(err, fs.ErrNotExist)
}
