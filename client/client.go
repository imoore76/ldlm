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
This file contains an easy-to-use client interface for LDLM.
*/

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/imoore76/go-ldlm/lock"
	pb "github.com/imoore76/go-ldlm/protos"
	"github.com/imoore76/go-ldlm/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Re-namespace errors here so they can be easily used by clients
var (
	ErrLockDoesNotExist             = lock.ErrLockDoesNotExist
	ErrInvalidLockKey               = lock.ErrInvalidLockKey
	ErrLockWaitTimeout              = server.ErrLockWaitTimeout
	ErrLockNotLocked                = lock.ErrLockNotLocked
	ErrLockDoesNotExistOrInvalidKey = server.ErrLockDoesNotExistOrInvalidKey
)

var minRefreshSeconds = uint32(10)

type Config struct {
	Address       string // host:port address of ldlm server
	NoAutoRefresh bool   // Don't automatically refresh locks before they expire
	UseTls        bool   // use TLS to connect to the server
	SkipVerify    bool   // don't verify the server's certificate
	CAFile        string // file containing a CA certificate
	TlsCert       string // file containing a TLS certificate for this client
	TlsKey        string // file containing a TLS key for this client
	Password      string // password to send
}

// Simple lock struct returned to clients
type Lock struct {
	client *client
	Name   string
	Key    string
	Locked bool
}

// Unlock attempts to release the lock.
//
// Returns:
// - bool: True if the lock was successfully released, false otherwise.
// - error: An error if the lock release fails.
func (l *Lock) Unlock() (bool, error) {
	if !l.Locked {
		return false, ErrLockNotLocked
	} else {
		unlocked, err := l.client.Unlock(l.Name, l.Key)
		if err == nil && unlocked {
			l.Locked = false
		}
		return unlocked, err
	}
}

// Interface for connection Closer
type Closer interface {
	Close() error
}

type client struct {
	conn           Closer
	pbc            pb.LDLMClient
	ctx            context.Context
	refreshMap     map[string]*refresher
	refreshMapLock sync.Mutex
	noAutoRefresh  bool
}

// New creates a new client instance with the given configuration.
//
// Parameters:
// - ctx: The context.Context used for the client.
// - conf: The Config struct containing the client configuration.
//
// Returns:
// - *client: The newly created client instance.
// - error: An error if the client creation fails.
func New(ctx context.Context, conf *Config) (*client, error) {
	creds := insecure.NewCredentials()
	if conf.UseTls || conf.TlsCert != "" {
		tlsc := &tls.Config{
			ServerName:         strings.Split(conf.Address, ":")[0],
			InsecureSkipVerify: conf.SkipVerify,
		}
		if conf.TlsCert != "" {
			clientCert, err := tls.LoadX509KeyPair(conf.TlsCert, conf.TlsKey)
			if err != nil {
				return nil, fmt.Errorf("error loading TlsCert and TlsKey: %w", err)
			}
			tlsc.Certificates = []tls.Certificate{clientCert}
		}
		if conf.CAFile != "" {
			if cacert, err := os.ReadFile(conf.CAFile); err != nil {
				return nil, fmt.Errorf("failed to read CA certificate: %w", err)
			} else {
				certPool := x509.NewCertPool()
				if !certPool.AppendCertsFromPEM(cacert) {
					return nil, errors.New("unknown error adding CA certificate to x509.CertPool")
				}
				tlsc.RootCAs = certPool
			}
		}
		creds = credentials.NewTLS(tlsc)
	}

	conn, err := grpc.Dial(
		conf.Address,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, err
	}

	if conf.Password != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", conf.Password)
	}

	return &client{
		conn:           conn,
		pbc:            pb.NewLDLMClient(conn),
		ctx:            ctx,
		refreshMap:     make(map[string]*refresher),
		refreshMapLock: sync.Mutex{},
		noAutoRefresh:  conf.NoAutoRefresh,
	}, nil
}

// Lock attempts to acquire a lock with the given name and timeouts.
//
// Parameters:
// - name: The name of the lock to acquire.
// - waitTimeoutSeconds: The maximum time to wait for the lock in seconds. If nil, there is no wait timeout.
// - lockTimeoutSeconds: The duration the lock will be held in seconds. If nil, there is no lock timeout.
//
// Returns:
// - *Lock: A pointer to a Lock struct containing the name, key, and locked status of the lock.
// - error: An error if the lock acquisition fails.
func (c *client) Lock(name string, waitTimeoutSeconds *uint32, lockTimeoutSeconds *uint32) (*Lock, error) {
	r, err := c.pbc.Lock(c.ctx, &pb.LockRequest{
		Name:               name,
		WaitTimeoutSeconds: waitTimeoutSeconds,
		LockTimeoutSeconds: lockTimeoutSeconds,
	})
	if err != nil {
		return nil, err
	}

	if r.Locked {
		c.maybeCreateRefresher(r, lockTimeoutSeconds)
	}
	return &Lock{Name: name, Key: r.Key, Locked: r.Locked, client: c}, rpcErrorToError(r.Error)

}

// TryLock attempts to acquire the lock and immediately fails or succeeds.
//
// Parameters:
// - name: The name of the lock to acquire.
// - lockTimeoutSeconds: The duration the lock will be held in seconds. If nil, there is no lock timeout.
//
// Returns:
// - *Lock: A pointer to a Lock struct containing the name, key, and locked status of the lock.
// - error: An error if the lock acquisition fails.
func (c *client) TryLock(name string, lockTimeoutSeconds *uint32) (*Lock, error) {
	r, err := c.pbc.TryLock(c.ctx, &pb.TryLockRequest{
		Name:               name,
		LockTimeoutSeconds: lockTimeoutSeconds,
	})
	if err != nil {
		return nil, err
	}
	if r.Locked {
		c.maybeCreateRefresher(r, lockTimeoutSeconds)
	}
	return &Lock{Name: name, Key: r.Key, Locked: r.Locked, client: c}, rpcErrorToError(r.Error)
}

// Unlock attempts to release a lock with the given name and key.
//
// Parameters:
// - name: The name of the lock to release.
// - key: The key of the lock to release.
//
// Returns:
// - bool: True if the lock was successfully released, false otherwise.
// - error: An error if the lock release fails.
func (c *client) Unlock(name string, key string) (bool, error) {
	r, err := c.pbc.Unlock(c.ctx, &pb.UnlockRequest{
		Name: name,
		Key:  key,
	})
	if err != nil {
		return false, err
	}
	if r.Unlocked {
		c.maybeRemoveRefresher(name)
	}
	return r.Unlocked, rpcErrorToError(r.Error)
}

// RefreshLock attempts to refresh a lock with the given name, key, and lock timeout.
//
// Parameters:
// - name: The name of the lock to refresh.
// - key: The key of the lock to refresh.
// - lockTimeoutSeconds: The lock timeout in seconds.
//
// Returns:
// - *Lock: A pointer to a Lock struct containing the name, key, and locked status of the lock.
// - error: An error if the lock refresh fails.
func (c *client) RefreshLock(name string, key string, lockTimeoutSeconds uint32) (*Lock, error) {
	r, err := c.pbc.RefreshLock(c.ctx, &pb.RefreshLockRequest{
		Name:               name,
		Key:                key,
		LockTimeoutSeconds: lockTimeoutSeconds,
	})
	if err != nil {
		return nil, err
	}
	return &Lock{Name: name, Key: r.Key, Locked: r.Locked, client: c}, rpcErrorToError(r.Error)
}

// Close closes the client connection.
//
// No parameters.
// Returns an error if the connection close fails.
func (c *client) Close() error {
	c.refreshMapLock.Lock()
	defer c.refreshMapLock.Unlock()
	for _, refresher := range c.refreshMap {
		refresher.Stop()
	}

	return c.conn.Close()
}

// maybeCreateRefresher creates a refresher if the lock is locked, auto-refresh is enabled, and the
// lock timeout is not zero.
//
// Parameters:
// - r: A pointer to a LockResponse struct containing the lock information.
// - lockTimeoutSeconds: A pointer to a uint32 representing the lock timeout in seconds.
func (c *client) maybeCreateRefresher(r *pb.LockResponse, lockTimeoutSeconds *uint32) {
	if !r.Locked || c.noAutoRefresh || lockTimeoutSeconds == nil || *lockTimeoutSeconds == 0 {
		return
	}

	// Create and add lock to refresh map
	c.refreshMapLock.Lock()
	defer c.refreshMapLock.Unlock()

	if _, ok := c.refreshMap[r.Name]; ok {
		panic("client out of sync - lock already exists in refresh map")
	}

	c.refreshMap[r.Name] = NewRefresher(c, r.Name, r.Key, *lockTimeoutSeconds)
}

// maybeRemoveRefresher removes a refresher from the refresh map if auto-refresh is enabled and the
// refresher exists.
//
// Parameters:
// - name: The name of the refresher to remove.
//
// Return:
// - None.
func (c *client) maybeRemoveRefresher(name string) {
	if c.noAutoRefresh {
		return
	}

	c.refreshMapLock.Lock()
	refresher, ok := c.refreshMap[name]
	c.refreshMapLock.Unlock()

	if ok {
		delete(c.refreshMap, name)
		refresher.Stop()
	}
}

type refresher struct {
	client             *client
	name               string
	key                string
	lockTimeoutSeconds uint32
	stop               chan struct{}
}

// NewRefresher creates a new refresher instance with the given client, name, key, and lock timeout.
//
// Parameters:
// - client: A pointer to a client struct.
// - name: A string representing the name of the refresher.
// - key: A string representing the key of the refresher.
// - lockTimeoutSeconds: An unsigned 32-bit integer representing the lock timeout in seconds.
//
// Return:
// - A pointer to a refresher struct.
func NewRefresher(client *client, name string, key string, lockTimeoutSeconds uint32) *refresher {
	r := &refresher{
		client:             client,
		name:               name,
		key:                key,
		lockTimeoutSeconds: lockTimeoutSeconds,
		stop:               make(chan struct{}, 1),
	}
	r.Start()
	return r
}

// Start starts the refresher.
//
// It does not take any parameters.
// It does not return anything.
func (r *refresher) Start() {
	var interval uint32
	if r.lockTimeoutSeconds <= 30 {
		interval = minRefreshSeconds
	} else {
		// an unsigned int that is less than 30 would wrap here
		interval = max(r.lockTimeoutSeconds-30, minRefreshSeconds)
	}
	go func() {
		for {
			select {
			case <-r.client.ctx.Done():
				return
			case <-r.stop:
				return
			case <-time.After(time.Duration(interval) * time.Second):
				if _, err := r.client.RefreshLock(r.name, r.key, r.lockTimeoutSeconds); err != nil {
					panic("error refreshing lock " + r.name + " " + err.Error())
				}
			}
		}
	}()
}

// Stop stops the refresher by closing the stop channel.
//
// No parameters.
// No return values.
func (r *refresher) Stop() {
	close(r.stop)
}

// rpcErrorToError converts an RPC error to a standard error.
//
// Parameters:
// - err: A pointer to a pb.Error struct representing the RPC error.
//
// Returns:
// - error: A standard error representing the converted RPC error. If the input error is nil, nil is returned.
func rpcErrorToError(err *pb.Error) error {
	if err == nil {
		return nil
	}

	switch err.Code {
	case 0:
		return errors.New(err.Message)
	case 1:
		return ErrLockDoesNotExist
	case 2:
		return ErrInvalidLockKey
	case 3:
		return ErrLockWaitTimeout
	case 4:
		return ErrLockNotLocked
	case 5:
		return ErrLockDoesNotExistOrInvalidKey
	}

	return fmt.Errorf("unknown RPC error. code: %d message: %s", err.Code, err.Message)
}
