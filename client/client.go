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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Re-namespace errors here so they can be easily used by clients
var (
	ErrLockDoesNotExist             = lock.ErrLockDoesNotExist
	ErrInvalidLockKey               = lock.ErrInvalidLockKey
	ErrLockWaitTimeout              = server.ErrLockWaitTimeout
	ErrLockNotLocked                = lock.ErrLockNotLocked
	ErrLockDoesNotExistOrInvalidKey = server.ErrLockDoesNotExistOrInvalidKey
	ErrInvalidLockSize              = lock.ErrInvalidLockSize
	ErrLockSizeMismatch             = lock.ErrLockSizeMismatch
)

var (
	// Minimum amount of time to wait before renewing a lock
	minRenewSeconds = int32(10)
	// The delay between failed retries
	retryDelaySeconds = 3
)

type Config struct {
	Address     string // host:port address of ldlm server
	NoAutoRenew bool   // Don't automatically renew locks before they expire
	UseTls      bool   // use TLS to connect to the server
	SkipVerify  bool   // don't verify the server's certificate
	CAFile      string // file containing a CA certificate
	TlsCert     string // file containing a TLS certificate for this client
	TlsKey      string // file containing a TLS key for this client
	Password    string // password to send
	MaxRetries  int    // maximum number of retries on network error or server unreachable
}

// Simple lock struct returned to clients.
type Lock struct {
	client *client
	Name   string
	Key    string
	Locked bool
}

// Lock options struct.
type LockOptions struct {
	WaitTimeoutSeconds int32
	LockTimeoutSeconds int32
	Size               int32
}

// Unlock attempts to release the lock.
//
// Returns:
// - error: An error if the lock release fails.
func (l *Lock) Unlock() error {
	if !l.Locked {
		return ErrLockNotLocked
	} else {
		unlocked, err := l.client.Unlock(l.Name, l.Key)
		l.Locked = !(err == nil && unlocked)
		return err
	}
}

// Renew attempts to renew the lock.
//
// Parameters:
// - lockTimeoutSeconds: The lock timeout in seconds.
//
// Returns:
// - error: An error if the lock renew fails.
func (l *Lock) Renew(lockTimeoutSeconds int32) error {
	if !l.Locked {
		return ErrLockNotLocked
	} else {
		_, err := l.client.Renew(l.Name, l.Key, lockTimeoutSeconds)
		return err
	}
}

// Interface for connection Closer
type Closer interface {
	Close() error
}

type client struct {
	conn        Closer
	pbc         pb.LDLMClient
	ctx         context.Context
	renewMap    sync.Map
	noAutoRenew bool
	maxRetries  int
}

// New creates a new client instance with the given configuration.
//
// Parameters:
// - ctx: The context.Context used for the client.
// - conf: The Config struct containing the client configuration.
// - opts: Optional grpc.DialOptions for the client.
//
// Returns:
// - *client: The newly created client instance.
// - error: An error if the client creation fails.
func New(ctx context.Context, conf Config, opts ...grpc.DialOption) (*client, error) {
	creds := insecure.NewCredentials()
	if conf.UseTls || conf.TlsCert != "" {
		tlsC := &tls.Config{
			ServerName:         strings.Split(conf.Address, ":")[0],
			InsecureSkipVerify: conf.SkipVerify,
		}
		if conf.TlsCert != "" {
			clientCert, err := tls.LoadX509KeyPair(conf.TlsCert, conf.TlsKey)
			if err != nil {
				return nil, fmt.Errorf("error loading TlsCert and TlsKey: %w", err)
			}
			tlsC.Certificates = []tls.Certificate{clientCert}
		}
		if conf.CAFile != "" {
			if cacert, err := os.ReadFile(conf.CAFile); err != nil {
				return nil, fmt.Errorf("failed to read CA certificate: %w", err)
			} else {
				certPool := x509.NewCertPool()
				if !certPool.AppendCertsFromPEM(cacert) {
					return nil, errors.New("unknown error adding CA certificate to x509.CertPool")
				}
				tlsC.RootCAs = certPool
			}
		}
		creds = credentials.NewTLS(tlsC)
	}

	opts = append(opts, grpc.WithTransportCredentials(creds))
	conn, err := grpc.NewClient(
		conf.Address,
		opts...,
	)
	if err != nil {
		return nil, err
	}

	if conf.Password != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", conf.Password)
	}

	return &client{
		conn:        conn,
		pbc:         pb.NewLDLMClient(conn),
		ctx:         ctx,
		renewMap:    sync.Map{},
		noAutoRenew: conf.NoAutoRenew,
		maxRetries:  conf.MaxRetries,
	}, nil
}

// Lock attempts to acquire a lock with the given name and timeouts.
//
// Parameters:
// - name: The name of the lock to acquire.
// - LockOptions: The LockOptions struct containing the lock options.
//
// Returns:
// - *Lock: A pointer to a Lock struct containing the name, key, and locked status of the lock.
// - error: An error if the lock acquisition fails.
func (c *client) Lock(name string, o *LockOptions) (*Lock, error) {
	if o == nil {
		o = &LockOptions{}
	}

	req := &pb.LockRequest{
		Name: name,
	}
	if o.WaitTimeoutSeconds > 0 {
		req.WaitTimeoutSeconds = &o.WaitTimeoutSeconds
	}
	if o.LockTimeoutSeconds > 0 {
		req.LockTimeoutSeconds = &o.LockTimeoutSeconds
	}
	if o.Size > 0 {
		req.Size = &o.Size
	}
	resp, err := rpcWithRetry(
		c.maxRetries,
		func() (*pb.LockResponse, error) {
			return c.pbc.Lock(c.ctx, req)
		},
	)
	if err != nil {
		return nil, err
	}

	if resp.Locked {
		c.maybeCreateRenewer(resp, o.LockTimeoutSeconds)
	}
	return &Lock{
		Name:   resp.Name,
		Key:    resp.Key,
		Locked: resp.Locked,
		client: c,
	}, rpcErrorToError(resp.Error)

}

// TryLock attempts to acquire the lock and immediately fails or succeeds.
//
// Parameters:
// - name: The name of the lock to acquire.
// - LockOptions: The LockOptions struct containing the lock options.
//
// Returns:
// - *Lock: A pointer to a Lock struct containing the name, key, and locked status of the lock.
// - error: An error if the lock acquisition fails.
func (c *client) TryLock(name string, o *LockOptions) (*Lock, error) {
	if o == nil {
		o = &LockOptions{}
	}

	if o.WaitTimeoutSeconds > 0 {
		return nil, errors.New("wait timeout not supported for TryLock")
	}
	req := &pb.TryLockRequest{
		Name: name,
	}
	if o.LockTimeoutSeconds > 0 {
		req.LockTimeoutSeconds = &o.LockTimeoutSeconds
	}
	if o.Size > 0 {
		req.Size = &o.Size
	}
	resp, err := rpcWithRetry(c.maxRetries, func() (*pb.LockResponse, error) {
		return c.pbc.TryLock(c.ctx, req)
	})
	if err != nil {
		return nil, err
	}
	if resp.Locked {
		c.maybeCreateRenewer(resp, o.LockTimeoutSeconds)
	}
	return &Lock{
		Name:   resp.Name,
		Key:    resp.Key,
		Locked: resp.Locked,
		client: c,
	}, rpcErrorToError(resp.Error)
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
	c.maybeRemoveRenewer(name)
	r, err := rpcWithRetry(
		c.maxRetries,
		func() (*pb.UnlockResponse, error) {
			return c.pbc.Unlock(c.ctx, &pb.UnlockRequest{
				Name: name,
				Key:  key,
			})
		},
	)
	if err != nil {
		return false, err
	}
	return r.Unlocked, rpcErrorToError(r.Error)
}

// Renew attempts to renew a lock with the given name, key, and lock timeout.
//
// Parameters:
// - name: The name of the lock to renew.
// - key: The key of the lock to renew.
// - lockTimeoutSeconds: The lock timeout in seconds.
//
// Returns:
// - *Lock: A pointer to a Lock struct containing the name, key, and locked status of the lock.
// - error: An error if the lock renew fails.
func (c *client) Renew(name string, key string, lockTimeoutSeconds int32) (*Lock, error) {
	r, err := rpcWithRetry(
		c.maxRetries,
		func() (*pb.LockResponse, error) {
			return c.pbc.Renew(c.ctx, &pb.RenewRequest{
				Name: name, Key: key, LockTimeoutSeconds: lockTimeoutSeconds,
			})
		},
	)
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
	c.renewMap.Range(func(k, v interface{}) bool {
		renewer := v.(*renewer)
		renewer.Stop()
		return true
	})

	return c.conn.Close()
}

// maybeCreateRenewer creates a renewer if the lock is locked, auto-renew is enabled, and the
// lock timeout is not zero.
//
// Parameters:
// - r: A pointer to a LockResponse struct containing the lock information.
// - lockTimeoutSeconds: A int32 representing the lock timeout in seconds.
func (c *client) maybeCreateRenewer(r *pb.LockResponse, lockTimeoutSeconds int32) {
	if !r.Locked || c.noAutoRenew || lockTimeoutSeconds == 0 {
		return
	}

	// Create and add lock to renew map
	rFresher := NewRenewer(c, r.Name, r.Key, lockTimeoutSeconds)
	if _, loaded := c.renewMap.LoadOrStore(r.Name, rFresher); loaded {
		panic("client out of sync - lock already exists in renew map")
	}
}

// maybeRemoveRenewer removes a renewer from the renew map if auto-renew is enabled and the
// renewer exists.
//
// Parameters:
// - name: The name of the renewer to remove.
//
// Return:
// - None.
func (c *client) maybeRemoveRenewer(name string) {
	if c.noAutoRenew {
		return
	}

	r, ok := c.renewMap.LoadAndDelete(name)

	if ok {
		r.(*renewer).Stop()
	}
}

type renewer struct {
	client             *client
	name               string
	key                string
	lockTimeoutSeconds int32
	stop               chan struct{}
}

// NewRenewer creates a new renewer instance with the given client, name, key, and lock timeout.
//
// Parameters:
// - client: A pointer to a client struct.
// - name: A string representing the name of the renewer.
// - key: A string representing the key of the renewer.
// - lockTimeoutSeconds: An unsigned 32-bit integer representing the lock timeout in seconds.
//
// Return:
// - A pointer to a renewer struct.
func NewRenewer(client *client, name string, key string, lockTimeoutSeconds int32) *renewer {
	r := &renewer{
		client:             client,
		name:               name,
		key:                key,
		lockTimeoutSeconds: lockTimeoutSeconds,
		stop:               make(chan struct{}),
	}
	r.Start()
	return r
}

// Start starts the renewer.
//
// It does not take any parameters.
// It does not return anything.
func (r *renewer) Start() {
	var interval int32
	if r.lockTimeoutSeconds <= 30 {
		interval = minRenewSeconds
	} else {
		// an unsigned int that is less than 30 would wrap here
		interval = max(r.lockTimeoutSeconds-30, minRenewSeconds)
	}
	go func() {
		for {
			t := time.NewTimer(time.Duration(interval) * time.Second)
			select {
			case <-r.client.ctx.Done():
				if !t.Stop() {
					<-t.C
				}
				close(r.stop)
				return
			case <-r.stop:
				if !t.Stop() {
					<-t.C
				}
				close(r.stop)
				return
			case <-t.C:
				if _, err := r.client.Renew(r.name, r.key, r.lockTimeoutSeconds); err != nil {
					panic("error renewing lock " + r.name + " " + err.Error())
				}
			}
		}
	}()
}

// Stop stops the renewer by closing the stop channel.
//
// No parameters.
// No return values.
func (r *renewer) Stop() {
	select {
	case r.stop <- struct{}{}:
		<-r.stop
	default:
	}
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
	case pb.ErrorCode_Unknown:
		return errors.New(err.Message)
	case pb.ErrorCode_LockDoesNotExist:
		return ErrLockDoesNotExist
	case pb.ErrorCode_InvalidLockKey:
		return ErrInvalidLockKey
	case pb.ErrorCode_LockWaitTimeout:
		return ErrLockWaitTimeout
	case pb.ErrorCode_NotLocked:
		return ErrLockNotLocked
	case pb.ErrorCode_LockDoesNotExistOrInvalidKey:
		return ErrLockDoesNotExistOrInvalidKey
	case pb.ErrorCode_LockSizeMismatch:
		return ErrLockSizeMismatch
	case pb.ErrorCode_InvalidLockSize:
		return ErrInvalidLockSize
	}

	return fmt.Errorf("unknown RPC error. code: %d message: %s", err.Code, err.Message)
}

// rpcWithRetry performs an RPC call with retry logic.
//
// It takes two parameters:
// - maxRetries: an integer representing the maximum number of retries.
// - f: a function that performs the RPC call and returns a value of type T and an error.
//
// The function returns a value of type T and an error.
func rpcWithRetry[T any](maxRetries int, f func() (T, error)) (T, error) {

	var retries int = 0
	for {
		r, err := f()
		if err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
				if retries >= maxRetries {
					return r, err
				}
				retries++
				time.Sleep(time.Duration(retryDelaySeconds) * time.Second)
				continue
			} else {
				return r, err
			}

		} else {
			return r, nil
		}
	}
}
