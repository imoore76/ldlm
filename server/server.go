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
This file contains the server package's Run() function, config, and helpers
*/
package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	log "log/slog"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/imoore76/go-ldlm/protos"
	"github.com/imoore76/go-ldlm/server/locksrv"
)

// The struct required to configure the server. See the config package
type ServerConfig struct {
	KeepaliveInterval time.Duration `desc:"Interval at which to send keepalive pings to client" default:"60s" short:"k"`
	KeepaliveTimeout  time.Duration `desc:"Wait this duration for the ping ack before assuming the connection is dead" default:"10s" short:"t"`
	ListenAddress     string        `desc:"Address (host:port) at which to listen" default:"localhost:3144" short:"l"`
	TlsCert           string        `desc:"File containing TLS certificate" default:""`
	TlsKey            string        `desc:"File containing TLS key" default:""`
	ClientCertVerify  bool          `desc:"Verify client certificate" default:"false"`
	ClientCA          string        `desc:"File containing client CA certificate. This will also enable client cert verification." default:""`
	Password          string        `desc:"Password required of clients" default:""`
	locksrv.LockSrvConfig
}

// Run initializes and starts the gRPC server.
//
// It takes a configuration struct as input.
// Returns a function to shut down the server and an error.
func Run(conf *ServerConfig) (func(), error) {

	// protobuf interface server
	server, lsStopperFunc := locksrv.New(
		&conf.LockSrvConfig,
	)

	lis, err := net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		lsStopperFunc()
		return nil, err
	}

	grpcOpts := []grpc.ServerOption{
		// see locksrv/connection_handler.go
		grpc.StatsHandler(server),
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
	if creds, err := loadTLSCredentials(conf); err != nil {
		lsStopperFunc()
		return nil, err
	} else if creds != nil {
		grpcOpts = append(grpcOpts, grpc.Creds(creds))
		log.Info("Loaded TLS configuration")
	}

	// Add authentication if provided
	if conf.Password != "" {
		grpcOpts = append(grpcOpts, authPasswordInterceptor(conf.Password))
		log.Info("Loaded authentication configuration")
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	pb.RegisterLDLMServer(grpcServer, server)

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
		lsStopperFunc()
		// Can't GracefulStop() here or clients waiting for locks will block
		// the server from exiting.
		grpcServer.Stop()
	}, nil
}

// Return a credentials.TransportCredentials if TLS is configured
func loadTLSCredentials(conf *ServerConfig) (credentials.TransportCredentials, error) {
	useTls := false

	// Create tls config
	tlsConfig := &tls.Config{}

	// Load server's certificate and private key
	if conf.TlsCert != "" {
		serverCert, err := tls.LoadX509KeyPair(conf.TlsCert, conf.TlsKey)
		if err != nil {
			return nil, fmt.Errorf("LoadX509KeyPair() error loading cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{serverCert}
		useTls = true
	}

	// Client certificate verification
	if conf.ClientCA != "" {
		caPem, err := os.ReadFile(conf.ClientCA)
		if err != nil {
			return nil, fmt.Errorf("os.ReadFile() failed to read ca cert: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caPem) {
			return nil, fmt.Errorf("AppendCertsFromPEM() failed to append client ca cert")
		}
		tlsConfig.ClientCAs = certPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		useTls = true

	} else if conf.ClientCertVerify {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		useTls = true
	}

	// Can't do client TLS without server TLS
	if useTls && conf.TlsCert == "" {
		return nil, fmt.Errorf("client TLS certificate verification requires server TLS to be configured")
	}

	if useTls {
		return credentials.NewTLS(tlsConfig), nil
	}

	return nil, nil

}

// authPasswordInterceptor returns a gRPC interceptor for authentication
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
