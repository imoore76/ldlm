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

package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/imoore76/go-ldlm/config"
	con "github.com/imoore76/go-ldlm/constants"
	pb "github.com/imoore76/go-ldlm/protos"
	"github.com/imoore76/go-ldlm/server/locksrv"
)

func testingServer(conf *ServerConfig) (clientFactory func() (pb.LDLMClient, func()), closer func()) {
	// from https://medium.com/@3n0ugh/how-to-test-grpc-servers-in-go-ba90fe365a18
	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	tmpFile, _ := os.CreateTemp("", "ldlm-test-ipc-*")
	tmpFile.Close()

	conf.LockSrvConfig.IPCConfig.IPCSocketFile = tmpFile.Name()
	os.Remove(tmpFile.Name())

	server, lsStopperFunc := locksrv.New(&conf.LockSrvConfig)

	grpcOpts := []grpc.ServerOption{
		grpc.StatsHandler(server),
	}

	if conf.Password != "" {
		grpcOpts = append(grpcOpts, authPasswordInterceptor(conf.Password))
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	pb.RegisterLDLMServer(grpcServer, server)

	// Blocking call
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("error serving server: %v", err)
		}
	}()

	closer = func() {
		err := lis.Close()
		if err != nil {
			log.Printf("error closing listener: %v", err)
		}
		lsStopperFunc()
		grpcServer.Stop()
		os.Remove(tmpFile.Name())
	}

	// Factory function for generating clients connected to this server instance
	clientFactory = func() (pb.LDLMClient, func()) {
		conn, err := grpc.DialContext(context.Background(), "",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}), grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			panic(fmt.Sprintf("error connecting to server: %v", err))
		}
		// Return client and a client connection closer
		return pb.NewLDLMClient(conn), func() { conn.Close() }
	}

	return clientFactory, closer
}

func TestRun(t *testing.T) {
	conf := config.Configure[ServerConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)
	conf.ListenAddress = "127.0.0.1:0"

	shutdown, err := Run(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test that the function starts the gRPC server successfully
	if shutdown == nil {
		t.Error("expected non-nil shutdown function")
	}

	// Test that the function returns a function to gracefully shut down the
	// server and no error
	shutdown()
}

func TestRun_ListenError(t *testing.T) {
	conf := config.Configure[ServerConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)
	conf.ListenAddress = "x2"

	_, err := Run(conf)
	if err == nil {
		t.Error("expected error but got nil")
	} else if err.Error() != "listen tcp: address x2: missing port in address" {
		t.Errorf("expected %s but got nil %s", "listen tcp: address x2: missing port in address", err.Error())
	}
}

func TestRun_UseTLSConfig(t *testing.T) {
	cases := map[string]struct {
		args     []string
		expected bool
		err      string
	}{
		"cert_and_key": {
			args: []string{
				"--tls_cert", "../testcerts/server_cert.pem",
				"--tls_key", "../testcerts/server_key.pem",
			},
			expected: true,
		},
		"just_key": {
			args: []string{
				"--tls_key", "../testcerts/server_key.pem",
			},
			expected: false,
		},
		"client_cert_verify": {
			args: []string{
				"--client_cert_verify",
			},
			expected: false,
			err:      "client TLS certificate verification requires server TLS to be configured",
		},
		"client_ca": {
			args: []string{
				"--client_ca", "../testcerts/ca_cert.pem",
			},
			expected: false,
			err:      "client TLS certificate verification requires server TLS to be configured",
		},
		"none": {
			args:     []string{},
			expected: false,
		},
	}

	for name, c := range cases {
		conf := config.Configure[ServerConfig](
			&config.Options{EnvPrefix: con.TestConfigEnvPrefix, Args: c.args},
		)

		creds, err := loadTLSCredentials(conf)
		if c.err != "" {
			assert.EqualError(t, err, c.err, "Test case %s failed", name)
			continue
		} else {
			assert.Nil(t, err, "Test case %s failed", name)
		}
		assert.True(t, (creds != nil) == c.expected, "Test case %s failed", name)
	}
}

func TestRun_TLSOptions(t *testing.T) {
	assert := assert.New(t)

	conf := config.Configure[ServerConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)
	conf.TlsCert = "../testcerts/server_cert.pem"
	conf.TlsKey = "../testcerts/server_key.pem"
	conf.ClientCA = "../testcerts/client_ca_cert.pem"

	tcreds, err := loadTLSCredentials(conf)

	assert.NoError(err)
	assert.NotNil(tcreds)
}

func TestRun_PasswordNotSupplied(t *testing.T) {
	assert := assert.New(t)
	c := config.Configure[ServerConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)

	c.Password = "password"

	mkClient, closer := testingServer(c)
	defer closer()
	client, _ := mkClient()
	_, err := client.TryLock(context.Background(), &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.NotNil(err)
	assert.Equal(err.Error(), "rpc error: code = Unauthenticated desc = missing credentials")

}

func TestRun_PasswordWrong(t *testing.T) {
	assert := assert.New(t)
	c := config.Configure[ServerConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)
	c.Password = "password"

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "foo")

	mkClient, closer := testingServer(c)
	defer closer()
	client, _ := mkClient()
	_, err := client.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.NotNil(err)
	assert.Equal(err.Error(), "rpc error: code = Unauthenticated desc = invalid credentials")
}

func TestRun_Password(t *testing.T) {
	assert := assert.New(t)
	c := config.Configure[ServerConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)
	c.Password = "password"

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", c.Password)
	mkClient, closer := testingServer(c)
	defer closer()

	client, _ := mkClient()
	res, err := client.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)
}
