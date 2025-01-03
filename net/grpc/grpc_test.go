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

package grpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"testing"

	config "github.com/imoore76/configurature"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	con "github.com/imoore76/ldlm/constants"
	"github.com/imoore76/ldlm/lock"
	sec "github.com/imoore76/ldlm/net/security"
	pb "github.com/imoore76/ldlm/protos"
	"github.com/imoore76/ldlm/server"
	"github.com/imoore76/ldlm/timermap"
)

func getSconf() *sec.SecurityConfig {
	return config.Configure[sec.SecurityConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)
}

func TestRun(t *testing.T) {
	conf := config.Configure[GrpcConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)

	conf.ListenAddress = "127.0.0.1:0"

	stop, err := Run(NewService(&testLockServer{}), conf, getSconf())
	defer stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_ListenError(t *testing.T) {
	conf := config.Configure[GrpcConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)
	conf.ListenAddress = "x2"

	stop, err := Run(NewService(&testLockServer{}), conf, getSconf())
	if err == nil {
		t.Error("expected error but got nil")
		defer stop()
	} else if err.Error() != "listen tcp: address x2: missing port in address" {
		t.Errorf("expected %s but got nil %s", "listen tcp: address x2: missing port in address", err.Error())
	}
}

func TesPassword_NotSupplied(t *testing.T) {
	assert := assert.New(t)
	password := "password"

	mkClient, closer := testingServer(&testLockServer{}, password)
	defer closer()
	client, _ := mkClient()
	_, err := client.TryLock(context.Background(), &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.NotNil(err)
	assert.Equal(err.Error(), "rpc error: code = Unauthenticated desc = missing credentials")

}

func TestPassword_Invalid(t *testing.T) {
	assert := assert.New(t)
	password := "password"

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "foo")

	mkClient, closer := testingServer(&testLockServer{}, password)
	defer closer()
	client, _ := mkClient()
	_, err := client.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.NotNil(err)
	assert.Equal(err.Error(), "rpc error: code = Unauthenticated desc = invalid credentials")
}

func TestPassword(t *testing.T) {
	assert := assert.New(t)
	password := "password"

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", password)
	mkClient, closer := testingServer(&testLockServer{}, password)
	defer closer()

	client, _ := mkClient()
	res, err := client.TryLock(ctx, &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)
}

func TestLock(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	sz := int32(3)
	wto := int32(33)
	lto := int32(44)
	res, err := client.Lock(context.Background(), &pb.LockRequest{
		Name:               "testlock",
		WaitTimeoutSeconds: &wto,
		LockTimeoutSeconds: &lto,
		Size:               &sz,
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)

	assert.Equal(&struct {
		name               string
		lockTimeoutSeconds *int32
		waitTimeoutSeconds *int32
		size               *int32
	}{
		name:               "testlock",
		lockTimeoutSeconds: &lto,
		waitTimeoutSeconds: &wto,
		size:               &sz,
	}, l.lockCall)
}

func TestLock_Bare(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	res, err := client.Lock(context.Background(), &pb.LockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)

	assert.Equal(&struct {
		name               string
		lockTimeoutSeconds *int32
		waitTimeoutSeconds *int32
		size               *int32
	}{
		name:               "testlock",
		lockTimeoutSeconds: nil,
		waitTimeoutSeconds: nil,
		size:               nil,
	}, l.lockCall)
}

func TestLock_Error(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{
		lockResponse: &lockResponse{
			err: server.ErrEmptyName,
		},
	}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	res, err := client.Lock(context.Background(), &pb.LockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.NotNil(res.Error)
	if res.Error != nil {
		assert.Equal(server.ErrEmptyName.Error(), res.Error.Message)
	}
	assert.False(res.Locked)

	assert.Equal(&struct {
		name               string
		lockTimeoutSeconds *int32
		waitTimeoutSeconds *int32
		size               *int32
	}{
		name:               "testlock",
		lockTimeoutSeconds: nil,
		waitTimeoutSeconds: nil,
		size:               nil,
	}, l.lockCall)
}

func TestTryLock(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	sz := int32(3)
	lto := int32(44)
	res, err := client.TryLock(context.Background(), &pb.TryLockRequest{
		Name:               "testlock",
		LockTimeoutSeconds: &lto,
		Size:               &sz,
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)

	assert.Equal(&struct {
		name               string
		lockTimeoutSeconds *int32
		size               *int32
	}{
		name:               "testlock",
		lockTimeoutSeconds: &lto,
		size:               &sz,
	}, l.tryLockCall)
}

func TestTryLock_Bare(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	res, err := client.TryLock(context.Background(), &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)

	assert.Equal(&struct {
		name               string
		lockTimeoutSeconds *int32
		size               *int32
	}{
		name:               "testlock",
		lockTimeoutSeconds: nil,
		size:               nil,
	}, l.tryLockCall)
}

func TestTryLock_Error(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{
		tryLockResponse: &lockResponse{
			err: server.ErrEmptyName,
		},
	}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	res, err := client.TryLock(context.Background(), &pb.TryLockRequest{
		Name: "testlock",
	})
	assert.Nil(err)
	assert.NotNil(res.Error)
	if res.Error != nil {
		assert.Equal(server.ErrEmptyName.Error(), res.Error.Message)
	}
	assert.False(res.Locked)

	assert.Equal(&struct {
		name               string
		lockTimeoutSeconds *int32
		size               *int32
	}{
		name:               "testlock",
		lockTimeoutSeconds: nil,
		size:               nil,
	}, l.tryLockCall)
}

func TestRenew(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	res, err := client.Renew(context.Background(), &pb.RenewRequest{
		Name:               "testlock",
		Key:                "key",
		LockTimeoutSeconds: 23,
	})
	assert.Nil(err)
	assert.True(res.Locked)
	assert.Nil(res.Error)

	assert.Equal(&struct {
		name               string
		key                string
		lockTimeoutSeconds int32
	}{
		name:               "testlock",
		key:                "key",
		lockTimeoutSeconds: 23,
	}, l.renewCall)
}

func TestRenew_Error(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{
		renewResponse: &lockResponse{
			err: server.ErrEmptyName,
		},
	}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	res, err := client.Renew(context.Background(), &pb.RenewRequest{
		Name:               "testlock",
		Key:                "foo",
		LockTimeoutSeconds: 34,
	})
	assert.Nil(err)
	assert.NotNil(res.Error)
	if res.Error != nil {
		assert.Equal(server.ErrEmptyName.Error(), res.Error.Message)
	}
	assert.False(res.Locked)

	assert.Equal(&struct {
		name               string
		key                string
		lockTimeoutSeconds int32
	}{
		name:               "testlock",
		key:                "foo",
		lockTimeoutSeconds: 34,
	}, l.renewCall)
}

func TestUnlock(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	res, err := client.Unlock(context.Background(), &pb.UnlockRequest{
		Name: "testlock",
		Key:  "key",
	})
	assert.Nil(err)
	assert.True(res.Unlocked)
	assert.Nil(res.Error)

	assert.Equal(&struct {
		name string
		key  string
	}{
		name: "testlock",
		key:  "key",
	}, l.unlockCall)
}

func TestUnlock_Error(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{
		unlockResponse: &struct {
			unlocked bool
			err      error
		}{
			unlocked: false,
			err:      server.ErrEmptyName,
		},
	}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, _ := mkClient()
	res, err := client.Unlock(context.Background(), &pb.UnlockRequest{
		Name: "testlock",
		Key:  "foo",
	})
	assert.Nil(err)
	assert.NotNil(res.Error)
	if res.Error != nil {
		assert.Equal(server.ErrEmptyName.Error(), res.Error.Message)
	}
	assert.False(res.Unlocked)

	assert.Equal(&struct {
		name string
		key  string
	}{
		name: "testlock",
		key:  "foo",
	}, l.unlockCall)
}

func TestErrorsToProtoErrors(t *testing.T) {
	assert := assert.New(t)
	cases := map[string]struct {
		err  error
		code pb.ErrorCode
	}{
		"unknown error": {
			err:  errors.New("foo"),
			code: pb.ErrorCode_Unknown,
		},
		"wait timeout": {
			err:  server.ErrLockWaitTimeout,
			code: pb.ErrorCode_LockWaitTimeout,
		},
		"invalid lock key": {
			err:  lock.ErrInvalidLockKey,
			code: pb.ErrorCode_InvalidLockKey,
		},
		"lock does not exist": {
			err:  lock.ErrLockDoesNotExist,
			code: pb.ErrorCode_LockDoesNotExist,
		},
		"lock not locked": {
			err:  lock.ErrLockNotLocked,
			code: pb.ErrorCode_NotLocked,
		},
		"lock does not exist or invalid key": {
			err:  timermap.ErrTimerDoesNotExist,
			code: pb.ErrorCode_LockDoesNotExistOrInvalidKey,
		},
		"lock size mismatch": {
			err:  lock.ErrLockSizeMismatch,
			code: pb.ErrorCode_LockSizeMismatch,
		},
		"invalid lock size": {
			err:  lock.ErrInvalidLockSize,
			code: pb.ErrorCode_InvalidLockSize,
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(c.code, lockErrToProtoBuffErr(c.err).Code)
		})
		l := &testLockServer{
			unlockResponse: &struct {
				unlocked bool
				err      error
			}{
				unlocked: false,
				err:      c.err,
			},
			lockResponse: &lockResponse{
				err: c.err,
			},
			tryLockResponse: &lockResponse{
				err: c.err,
			},
			renewResponse: &lockResponse{
				err: c.err,
			},
		}

		mkClient, closer := testingServer(l, "")
		defer closer()

		client, _ := mkClient()
		res, err := client.Unlock(context.Background(), &pb.UnlockRequest{
			Name: "testlock",
			Key:  "foo",
		})
		assert.Nil(err)
		assert.NotNil(res.Error)
		if res.Error != nil {
			assert.Equal(c.code, res.Error.Code)
		}

		lres, err := client.Lock(context.Background(), &pb.LockRequest{
			Name: "testlock",
		})
		assert.Nil(err)
		assert.False(lres.Locked)
		assert.NotNil(lres.Error)
		if res.Error != nil {
			assert.Equal(c.code, lres.Error.Code)
		}

		lres, err = client.TryLock(context.Background(), &pb.TryLockRequest{
			Name: "testlock",
		})
		assert.Nil(err)
		assert.False(lres.Locked)
		assert.NotNil(res.Error)
		if res.Error != nil {
			assert.Equal(c.code, res.Error.Code)
		}

		lres, err = client.Renew(context.Background(), &pb.RenewRequest{
			Name: "testlock",
		})
		assert.Nil(err)
		assert.False(lres.Locked)
		assert.NotNil(res.Error)
		if res.Error != nil {
			assert.Equal(c.code, res.Error.Code)
		}

	}
}

func TestSession(t *testing.T) {
	assert := assert.New(t)

	l := &testLockServer{}
	mkClient, closer := testingServer(l, "")
	defer closer()

	client, clCloser := mkClient()
	client.Lock(context.Background(), &pb.LockRequest{
		Name: "testlock",
	})
	clCloser()

	assert.Equal(&struct {
		metadata map[string]interface{}
	}{
		metadata: map[string]interface{}{"remote_address": "bufconn"},
	}, l.createSessionCall)

	<-l.sessionDestroyed
	assert.Equal(&struct {
		sessionId string
	}{
		sessionId: "TESTSESSIONID",
	}, l.destroySessionCall, l.destroySessionCall)

}

type sessIdKeyType string

var sessIdKey = sessIdKeyType("sessionKey")

type lockResponse struct {
	lock *server.Lock
	err  error
}

type testLockServer struct {
	lockResponse    *lockResponse
	tryLockResponse *lockResponse
	unlockResponse  *struct {
		unlocked bool
		err      error
	}
	renewResponse *lockResponse
	lockCall      *struct {
		name               string
		lockTimeoutSeconds *int32
		waitTimeoutSeconds *int32
		size               *int32
	}
	tryLockCall *struct {
		name               string
		lockTimeoutSeconds *int32
		size               *int32
	}
	unlockCall *struct {
		name string
		key  string
	}
	renewCall *struct {
		name               string
		key                string
		lockTimeoutSeconds int32
	}
	createSessionCall *struct {
		metadata map[string]any
	}

	destroySessionCall *struct {
		sessionId string
	}
	sessionDestroyed chan struct{}
}

func (t *testLockServer) Lock(ctx context.Context, name string, size *int32, lockTimeoutSeconds *int32, waitTimeoutSeconds *int32) (*server.Lock, error) {
	t.lockCall = &struct {
		name               string
		lockTimeoutSeconds *int32
		waitTimeoutSeconds *int32
		size               *int32
	}{
		name:               name,
		lockTimeoutSeconds: lockTimeoutSeconds,
		waitTimeoutSeconds: waitTimeoutSeconds,
		size:               size,
	}
	if t.lockResponse != nil {
		return t.lockResponse.lock, t.lockResponse.err
	}
	return &server.Lock{
		Name:   name,
		Locked: true,
		Key:    "key",
	}, nil
}

func (t *testLockServer) TryLock(ctx context.Context, name string, size *int32, lockTimeoutSeconds *int32) (*server.Lock, error) {
	t.tryLockCall = &struct {
		name               string
		lockTimeoutSeconds *int32
		size               *int32
	}{
		name:               name,
		lockTimeoutSeconds: lockTimeoutSeconds,
		size:               size,
	}
	if t.tryLockResponse != nil {
		return t.tryLockResponse.lock, t.tryLockResponse.err
	}
	return &server.Lock{
		Name:   name,
		Locked: true,
		Key:    "key",
	}, nil

}
func (t *testLockServer) Unlock(ctx context.Context, name string, key string) (bool, error) {
	t.unlockCall = &struct {
		name string
		key  string
	}{
		name: name,
		key:  key,
	}
	if t.unlockResponse != nil {
		return t.unlockResponse.unlocked, t.unlockResponse.err
	}
	return true, nil
}

func (t *testLockServer) Renew(ctx context.Context, name string, key string, lockTimeoutSeconds int32) (*server.Lock, error) {
	t.renewCall = &struct {
		name               string
		key                string
		lockTimeoutSeconds int32
	}{
		name:               name,
		key:                key,
		lockTimeoutSeconds: lockTimeoutSeconds,
	}
	if t.renewResponse != nil {
		return t.renewResponse.lock, t.renewResponse.err
	}
	return &server.Lock{
		Name:   name,
		Locked: true,
		Key:    "key",
	}, nil

}
func (t *testLockServer) DestroySession(ctx context.Context) string {
	sessionId := ctx.Value(sessIdKey).(string)
	t.destroySessionCall = &struct {
		sessionId string
	}{
		sessionId: sessionId,
	}

	close(t.sessionDestroyed)
	return sessionId
}

func (t *testLockServer) CreateSession(ctx context.Context, metadata map[string]any) (string, context.Context) {
	sessionId := "TESTSESSIONID"
	t.createSessionCall = &struct {
		metadata map[string]any
	}{
		metadata: metadata,
	}
	t.sessionDestroyed = make(chan struct{})
	return sessionId, context.WithValue(ctx, sessIdKey, sessionId)

}

func testingServer(lsrv *testLockServer, password string) (clientFactory func() (pb.LDLMClient, func()), closer func()) {
	// from https://medium.com/@3n0ugh/how-to-test-grpc-servers-in-go-ba90fe365a18
	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	// protobuf interface
	service := NewService(lsrv)
	grpcOpts := []grpc.ServerOption{
		grpc.StatsHandler(service),
	}

	if password != "" {
		grpcOpts = append(grpcOpts, authPasswordInterceptor(password))
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	pb.RegisterLDLMServer(grpcServer, service)

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
		grpcServer.Stop()
	}

	// Factory function for generating clients connected to this server instance
	clientFactory = func() (pb.LDLMClient, func()) {
		conn, err := grpc.NewClient("localhost:0",
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
