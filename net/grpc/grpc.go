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
This file contains the gRPC server package's struct,  Run() function, and helpers
*/
package grpc

import (
	"context"
	log "log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	"github.com/imoore76/go-ldlm/lock"
	"github.com/imoore76/go-ldlm/net/security"
	pb "github.com/imoore76/go-ldlm/protos"
	"github.com/imoore76/go-ldlm/server"
	"github.com/imoore76/go-ldlm/timer"
)

type Service struct {
	LockServer lockServer
	pb.UnimplementedLDLMServer
}

// Lock locks a lock with the given name and lock timeout, and waits for the lock if necessary.
//
// Parameters:
// - ctx: the context.Context for the request.
// - req: the *pb.LockRequest
//
// Returns:
// - *pb.LockResponse: the response containing the name, key, locked
func (s *Service) Lock(ctx context.Context, req *pb.LockRequest) (*pb.LockResponse, error) {
	lk, err := s.LockServer.Lock(ctx, req.Name, req.Size, req.LockTimeoutSeconds, req.WaitTimeoutSeconds)
	if lk == nil {
		lk = new(server.Lock)
	}
	return &pb.LockResponse{
		Name:   req.Name,
		Key:    lk.Key,
		Locked: lk.Locked,
		Error:  lockErrToProtoBuffErr(err),
	}, nil
}

// Unlock unlocks a lock with the given name and key.
//
// Parameters:
// - ctx: the context.Context for the request.
// - req: the *pb.UnlockRequest containing the name and key of the lock to unlock.
//
// Returns:
// - *pb.UnlockResponse: the response containing the name, unlocked status, and error.
// - error: any error that occurred during the unlock attempt.
func (s *Service) Unlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	unlocked, err := s.LockServer.Unlock(ctx, req.Name, req.Key)
	return &pb.UnlockResponse{
		Name:     req.Name,
		Unlocked: unlocked,
		Error:    lockErrToProtoBuffErr(err),
	}, nil
}

// TryLock attempts to acquire a lock with the given name and lock timeout.
//
// Parameters:
// - ctx: the context.Context for the request.
// - req: the *pb.TryLockRequest
//
// Returns:
// - *pb.LockResponse: the response containing the name, key, locked status, and error.
// - error: any error that occurred during the lock attempt.
func (s *Service) TryLock(ctx context.Context, req *pb.TryLockRequest) (*pb.LockResponse, error) {
	lk, err := s.LockServer.TryLock(ctx, req.Name, req.Size, req.LockTimeoutSeconds)
	if lk == nil {
		lk = new(server.Lock)
	}
	return &pb.LockResponse{
		Name:   req.Name,
		Key:    lk.Key,
		Locked: lk.Locked,
		Error:  lockErrToProtoBuffErr(err),
	}, nil

}

// RefreshLock refreshes the lock for a given name and key with a new lock timeout.
//
// Parameters:
// - ctx: the context.Context for the request.
// - req: the *pb.RefreshLockRequest containing the name, key, and lock timeout.
//
// Returns:
// - *pb.LockResponse: the response containing the name, key, locked status, and error.
// - error: any error that occurred during the refresh.
func (s *Service) RefreshLock(ctx context.Context, req *pb.RefreshLockRequest) (*pb.LockResponse, error) {
	lk, err := s.LockServer.RefreshLock(ctx, req.Name, req.Key, req.LockTimeoutSeconds)
	if lk == nil {
		lk = new(server.Lock)
	}
	return &pb.LockResponse{
		Name:   req.Name,
		Key:    lk.Key,
		Locked: lk.Locked,
		Error:  lockErrToProtoBuffErr(err),
	}, nil

}

// HandleConn is a function that handles a connection changes in the Service.
//
// It takes in a context and a stats.ConnStats as parameters.
// It does not return anything.
func (s *Service) HandleConn(ctx context.Context, st stats.ConnStats) {
	switch st.(type) {

	// Client disconnect
	case *stats.ConnEnd:
		s.LockServer.DestroySession(ctx)
	}
}

// TagConn is a function that is called with each new connection.
//
// It takes in a context and a ConnTagInfo struct as parameters.
// It returns a modified context.
func (s *Service) TagConn(ctx context.Context, st *stats.ConnTagInfo) context.Context {
	_, ctx = s.LockServer.CreateSession(ctx, map[string]any{
		"remote_address": st.RemoteAddr.String(),
	})
	return ctx
}

// Unimplemented methods, but needed to satisfy grpc.stats.Handler interface
func (s *Service) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context { return ctx }

func (s *Service) HandleRPC(ctx context.Context, _ stats.RPCStats) {}

// NewService returns a new Service with the given lockServer.
func NewService(srv lockServer) *Service {
	// protobuf interface
	return &Service{
		LockServer: srv,
	}
}

// Run initializes and starts the gRPC server.
//
// It takes a configuration struct as input.
// Returns a function to shut down the server and an error.
func Run(srv *Service, conf *GrpcConfig, sconf *security.SecurityConfig) (func(), error) {

	lis, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		return nil, err
	}

	grpcOpts := []grpc.ServerOption{
		// see locksrv/connection_handler.go
		grpc.StatsHandler(srv),
		// Allow clients to stay connected indefinitely. Client disconnects
		// could release all of their held locks
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 0 * time.Second, // Never send a GOAWAY for being idle
			MaxConnectionAge:  0 * time.Second, // Never send a GOAWAY for max connection age
			Time:              conf.KeepaliveInterval,
			Timeout:           conf.KeepaliveTimeout,
		}),
		// Keepalive configuration to detect dead clients
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second, // If a client pings more than once every 10 seconds, terminate the connection
			PermitWithoutStream: true,             // Allow pings even when there are no active streams
		}),
	}

	// Add TLS if provided
	if tlsConfig, err := security.GetTLSConfig(sconf); err != nil {
		return nil, err
	} else if tlsConfig != nil {
		creds := credentials.NewTLS(tlsConfig)
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
		log.Info("Loaded gRPC TLS configuration")
	}

	// Add authentication if provided
	if sconf.Password != "" {
		grpcOpts = append(grpcOpts, authPasswordInterceptor(sconf.Password))
		log.Info("Loaded gRPC authentication configuration")
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	pb.RegisterLDLMServer(grpcServer, srv)

	// The main server function, which blocks
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			// ErrServerStopped happens as part of normal grpcServer shutdown
			if err != grpc.ErrServerStopped {
				panic("error starting grpc server: " + err.Error())
			}
		}
	}()

	log.Warn("gRPC server started. Listening on " + conf.ListenAddress)

	// Return a function to properly shut down the server
	return func() {
		log.Warn("Shutting down...")
		// Can't GracefulStop() here or clients waiting for locks will block
		// the server from exiting.
		grpcServer.Stop()
	}, nil
}

// authPasswordInterceptor returns a gRPC interceptor for password authentication
func authPasswordInterceptor(password string) grpc.ServerOption {

	return grpc.UnaryInterceptor(
		func(ctx context.Context, r interface{}, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (interface{}, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok || md["authorization"] == nil {
				return nil, status.Errorf(codes.Unauthenticated, "missing credentials")
			}
			if md["authorization"][0] != password {
				return nil, status.Errorf(codes.Unauthenticated, "invalid credentials")
			}
			return h(ctx, r)
		},
	)
}

// lockErrToProtoBuffErr converts an error to a protobuf error
func lockErrToProtoBuffErr(e error) *pb.Error {
	if e == nil {
		return nil
	}

	var errCode pb.ErrorCode = pb.ErrorCode_Unknown
	switch e {
	case server.ErrLockWaitTimeout:
		errCode = pb.ErrorCode_LockWaitTimeout
	case lock.ErrInvalidLockKey:
		errCode = pb.ErrorCode_InvalidLockKey
	case lock.ErrLockDoesNotExist:
		errCode = pb.ErrorCode_LockDoesNotExist
	case lock.ErrLockNotLocked:
		errCode = pb.ErrorCode_NotLocked
	case timer.ErrTimerDoesNotExist:
		errCode = pb.ErrorCode_LockDoesNotExistOrInvalidKey
	case lock.ErrInvalidLockSize:
		errCode = pb.ErrorCode_InvalidLockSize
	case lock.ErrLockSizeMismatch:
		errCode = pb.ErrorCode_LockSizeMismatch
	}
	return &pb.Error{
		Code:    errCode,
		Message: e.Error(),
	}

}
