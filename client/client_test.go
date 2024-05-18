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
This file contains LDLM client tests
*/

package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	pb "github.com/imoore76/go-ldlm/protos"
	"github.com/stretchr/testify/assert"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLock_HappyPath(t *testing.T) {
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{
		NoAutoRefresh: true,
	}, gClient)

	l, err := c.Lock("test", nil, nil)

	assert := assert.New(t)
	assert.Nil(err)
	assert.True(l.Locked)
	assert.Equal("test", l.Name)

	assert.Empty(c.refreshMap)

	l.Unlock()

	assert.Equal([]*pb.LockRequest{{Name: "test"}}, gClient.lockRequests)
	assert.Equal([]*pb.UnlockRequest{{Name: "test", Key: l.Key}}, gClient.unlockRequests)
	assert.Empty(gClient.refreshLockRequests)
	assert.Empty(gClient.tryLockRequests)

}

func TestLock_LockTimeout(t *testing.T) {
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{
		NoAutoRefresh: true,
	}, gClient)

	to := uint32(24)
	l, err := c.Lock("test", nil, &to)

	assert := assert.New(t)
	assert.Nil(err)
	assert.True(l.Locked)
	assert.Equal("test", l.Name)
	assert.Empty(c.refreshMap)

	l.Unlock()

	assert.Equal([]*pb.LockRequest{{Name: "test", LockTimeoutSeconds: &to}}, gClient.lockRequests)
	assert.Equal([]*pb.UnlockRequest{{Name: "test", Key: l.Key}}, gClient.unlockRequests)
	assert.Empty(gClient.refreshLockRequests)
	assert.Empty(gClient.tryLockRequests)
}

func TestLock_LockTimeoutAutoRefresh(t *testing.T) {
	defer patchMinRefreshSeconds(2)()
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{}, gClient)

	to := uint32(5)
	l, err := c.Lock("test", nil, &to)

	assert := assert.New(t)
	assert.Nil(err)
	assert.True(l.Locked)
	assert.Equal("test", l.Name)

	time.Sleep(time.Duration(6) * time.Second)
	l.Unlock()

	assert.Equal([]*pb.LockRequest{{Name: "test", LockTimeoutSeconds: &to}}, gClient.lockRequests)
	assert.Equal([]*pb.RefreshLockRequest{
		{Name: l.Name, Key: l.Key, LockTimeoutSeconds: to},
		{Name: l.Name, Key: l.Key, LockTimeoutSeconds: to},
	}, gClient.refreshLockRequests)
	assert.Equal([]*pb.UnlockRequest{{Name: "test", Key: l.Key}}, gClient.unlockRequests)
	assert.Empty(gClient.tryLockRequests)

}

func TestLock_LockTimeoutAutoRefreshNotLocked(t *testing.T) {
	gClient := newTestGrpcClient([]*pb.LockResponse{
		{Name: "test", Locked: false, Error: &pb.Error{Code: 3}},
	}, nil, nil, nil)
	c := newTestClient(&Config{}, gClient)

	to := uint32(24)
	l, err := c.Lock("test", &to, &to)

	assert := assert.New(t)
	assert.ErrorIs(err, ErrLockWaitTimeout)
	assert.False(l.Locked)
	assert.Equal("test", l.Name)

	assert.Empty(c.refreshMap)

	assert.Equal([]*pb.LockRequest{{Name: "test", WaitTimeoutSeconds: &to, LockTimeoutSeconds: &to}}, gClient.lockRequests)
	assert.Empty(gClient.refreshLockRequests)
	assert.Empty(gClient.unlockRequests)
	assert.Empty(gClient.tryLockRequests)
}

func TestLock_WaitTimeout(t *testing.T) {
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{
		NoAutoRefresh: true,
	}, gClient)

	to := uint32(43)
	l, err := c.Lock("test", &to, nil)

	assert := assert.New(t)
	assert.Nil(err)
	assert.True(l.Locked)

	l.Unlock()

	assert.Equal([]*pb.LockRequest{{Name: "test", WaitTimeoutSeconds: &to}}, gClient.lockRequests)
	assert.Equal([]*pb.UnlockRequest{{Name: "test", Key: l.Key}}, gClient.unlockRequests)
	assert.Empty(gClient.tryLockRequests)
	assert.Empty(gClient.refreshLockRequests)
}

func TestLock_WaitTimeoutError(t *testing.T) {
	gClient := newTestGrpcClient([]*pb.LockResponse{
		{Name: "test", Locked: false, Error: &pb.Error{Code: 3}},
	}, nil, nil, nil)
	c := newTestClient(&Config{
		NoAutoRefresh: true,
	}, gClient)

	to := uint32(43)
	l, err := c.Lock("test", &to, nil)

	assert := assert.New(t)
	assert.ErrorIs(err, ErrLockWaitTimeout)
	assert.False(l.Locked)

	assert.Equal([]*pb.LockRequest{{Name: "test", WaitTimeoutSeconds: &to}}, gClient.lockRequests)
	assert.Empty(gClient.unlockRequests)
	assert.Empty(gClient.tryLockRequests)
	assert.Empty(gClient.refreshLockRequests)
}

func TestTryLock_HappyPath(t *testing.T) {
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{
		NoAutoRefresh: true,
	}, gClient)

	l, err := c.TryLock("test", nil)

	assert := assert.New(t)
	assert.Nil(err)
	assert.True(l.Locked)
	assert.Equal("test", l.Name)

	l.Unlock()

	assert.Equal([]*pb.TryLockRequest{{Name: "test"}}, gClient.tryLockRequests)
	assert.Equal([]*pb.UnlockRequest{{Name: "test", Key: l.Key}}, gClient.unlockRequests)
	assert.Empty(gClient.refreshLockRequests)
	assert.Empty(gClient.lockRequests)

}

func TestTryLock_LockTimeout(t *testing.T) {
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{
		NoAutoRefresh: true,
	}, gClient)

	to := uint32(24)
	l, err := c.TryLock("test", &to)

	assert := assert.New(t)
	assert.Empty(c.refreshMap)
	assert.Nil(err)
	assert.True(l.Locked)
	assert.Equal("test", l.Name)

	l.Unlock()

	assert.Equal([]*pb.TryLockRequest{{Name: "test", LockTimeoutSeconds: &to}}, gClient.tryLockRequests)
	assert.Equal([]*pb.UnlockRequest{{Name: "test", Key: l.Key}}, gClient.unlockRequests)
	assert.Empty(gClient.refreshLockRequests)
	assert.Empty(gClient.lockRequests)
}

func TestTryLock_LockTimeoutAutoRefresh(t *testing.T) {
	defer patchMinRefreshSeconds(2)()
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{}, gClient)

	to := uint32(24)
	l, err := c.TryLock("test", &to)

	assert := assert.New(t)
	assert.Nil(err)
	assert.True(l.Locked)
	assert.Equal("test", l.Name)

	time.Sleep(time.Duration(5) * time.Second)
	l.Unlock()

	assert.Equal([]*pb.TryLockRequest{{Name: "test", LockTimeoutSeconds: &to}}, gClient.tryLockRequests)
	assert.Equal([]*pb.RefreshLockRequest{
		{Name: l.Name, Key: l.Key, LockTimeoutSeconds: to},
		{Name: l.Name, Key: l.Key, LockTimeoutSeconds: to},
	}, gClient.refreshLockRequests)
	assert.Equal([]*pb.UnlockRequest{{Name: "test", Key: l.Key}}, gClient.unlockRequests)
	assert.Empty(gClient.lockRequests)

}

func TestTryLock_LockTimeoutAutoRefreshNotLocked(t *testing.T) {
	gClient := newTestGrpcClient(nil, []*pb.LockResponse{
		{Name: "test", Locked: false, Error: &pb.Error{Code: 3}},
	}, nil, nil)
	c := newTestClient(&Config{}, gClient)

	to := uint32(24)
	l, err := c.TryLock("test", &to)

	assert := assert.New(t)
	assert.NotNil(err)
	assert.ErrorIs(err, ErrLockWaitTimeout)
	assert.False(l.Locked)
	assert.Equal("test", l.Name)

	assert.Empty(c.refreshMap)

	l.Unlock()
	assert.Equal([]*pb.TryLockRequest{{Name: "test", LockTimeoutSeconds: &to}}, gClient.tryLockRequests)
	assert.Empty(gClient.refreshLockRequests)
	assert.Empty(gClient.unlockRequests)
	assert.Empty(gClient.lockRequests)
}

func TestTryLock_Error(t *testing.T) {
	gClient := newTestGrpcClient(nil, []*pb.LockResponse{
		{Name: "test", Locked: false, Error: &pb.Error{Code: 3}},
	}, nil, nil)
	c := newTestClient(&Config{}, gClient)

	to := uint32(24)
	l, err := c.TryLock("test", &to)

	assert := assert.New(t)
	assert.NotNil(err)
	assert.ErrorIs(err, ErrLockWaitTimeout)
	assert.False(l.Locked)
	assert.Equal("test", l.Name)
	l.Unlock()

	assert.Empty(c.refreshMap)

	assert.Equal([]*pb.TryLockRequest{{Name: "test", LockTimeoutSeconds: &to}}, gClient.tryLockRequests)
	assert.Empty(gClient.refreshLockRequests)
	assert.Empty(gClient.unlockRequests)
	assert.Empty(gClient.lockRequests)
}

func TestUnlock_Error(t *testing.T) {
	gClient := newTestGrpcClient(nil, nil, nil, []*pb.UnlockResponse{
		{Name: "test", Error: &pb.Error{Code: 2}},
	})
	c := newTestClient(&Config{}, gClient)

	unlocked, err := c.Unlock("test", "foo")

	assert := assert.New(t)

	assert.NotNil(err)
	assert.ErrorIs(err, ErrInvalidLockKey)
	assert.False(unlocked)
}

func TestNewClient_BareOptions(t *testing.T) {

	assert := assert.New(t)
	conf := &Config{
		Address: "127.0.0.1:8080",
	}
	ctx := context.Background()
	c, err := New(context.Background(), conf)
	ctx.Done()
	assert.Nil(err)
	assert.NotNil(c)
}

func TestNewClient_FullOptions(t *testing.T) {

	assert := assert.New(t)
	conf := &Config{
		Address:       "127.0.0.1:8080",
		NoAutoRefresh: true,
		UseTls:        true,
		SkipVerify:    true,
		CAFile:        "../testcerts/ca_cert.pem",
		TlsCert:       "../testcerts/client_cert.pem",
		TlsKey:        "../testcerts/client_key.pem",
		Password:      "password",
		MaxRetries:    3,
	}
	ctx := context.Background()
	c, err := New(context.Background(), conf)
	ctx.Done()
	assert.Nil(err)
	assert.NotNil(c)
}

func TestNewClient_BadCertOption(t *testing.T) {

	assert := assert.New(t)
	conf := &Config{
		Address: "127.0.0.1:8080",
		CAFile:  "../testcerts/ca_cert.pem",
		TlsCert: "../testcerts/client_cert.pem",
		TlsKey:  "../testcerts/client_cert.pem",
	}
	ctx := context.Background()
	c, err := New(context.Background(), conf)
	ctx.Done()
	assert.NotNil(err)
	assert.Nil(c)
}
func TestUnlock_StopRefresh(t *testing.T) {
	defer patchMinRefreshSeconds(1)()
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{}, gClient)
	r := NewRefresher(c, "test", "foo", 30)
	c.refreshMap.Store("test", r)
	time.Sleep(time.Duration(1500) * time.Millisecond)
	c.Unlock("test", "foo")
	time.Sleep(time.Duration(2000) * time.Millisecond)

	_, ok := c.refreshMap.Load("test")
	assert.False(t, ok)
	assert.Len(t, gClient.refreshLockRequests, 1)

}

func TestClose(t *testing.T) {
	defer patchMinRefreshSeconds(1)()
	gClient := newTestGrpcClient(nil, nil, nil, nil)
	c := newTestClient(&Config{}, gClient)

	r := NewRefresher(c, "test", "foo", 30)
	c.refreshMap.Store("test", r)
	time.Sleep(time.Duration(1500) * time.Millisecond)
	c.Close()
	time.Sleep(time.Duration(2000) * time.Millisecond)

	assert := assert.New(t)
	closer, _ := c.conn.(*closer)
	assert.True(closer.called)
	assert.Len(gClient.refreshLockRequests, 1)

}

func TestRpcWithRetry(t *testing.T) {
	defer patchRetryDelaySeconds(0)()

	responses := []struct {
		res *pb.LockResponse
		err error
	}{
		{nil, status.New(codes.Unavailable, "blah").Err()},
		{nil, status.New(codes.Unavailable, "blah2").Err()},
		{&pb.LockResponse{Locked: true, Name: "foo"}, nil},
	}

	var f = func() (*pb.LockResponse, error) {
		r := responses[0]
		responses = responses[1:]
		return r.res, r.err
	}
	assert := assert.New(t)

	l, err := rpcWithRetry(4, f)

	assert.Nil(err)
	assert.True(l.Locked)
	assert.Equal("foo", l.Name)
}

func TestRpcWithRetry_MaxRetries(t *testing.T) {
	defer patchRetryDelaySeconds(0)()

	responses := []struct {
		res *pb.LockResponse
		err error
	}{
		{nil, status.New(codes.Unavailable, "blah").Err()},
		{nil, status.New(codes.Unavailable, "blah2").Err()},
		{nil, status.New(codes.Unavailable, "blah3").Err()},
	}

	var f = func() (*pb.LockResponse, error) {
		r := responses[0]
		responses = responses[1:]
		return r.res, r.err
	}
	assert := assert.New(t)

	l, err := rpcWithRetry(2, f)

	assert.Nil(l)
	assert.NotNil(err)
	assert.Equal("rpc error: code = Unavailable desc = blah3", err.Error())
}

func TestRpcWithRetry_OtherError(t *testing.T) {
	defer patchRetryDelaySeconds(0)()

	responses := []struct {
		res *pb.LockResponse
		err error
	}{
		{nil, errors.New("ahhh")},
	}

	var f = func() (*pb.LockResponse, error) {
		r := responses[0]
		responses = responses[1:]
		return r.res, r.err
	}
	assert := assert.New(t)

	l, err := rpcWithRetry(2, f)

	assert.Nil(l)
	assert.NotNil(err)
	assert.Equal("ahhh", err.Error())
}

func TestRpcErrorToError(t *testing.T) {
	codeMap := map[int32]error{
		1: ErrLockDoesNotExist,
		2: ErrInvalidLockKey,
		3: ErrLockWaitTimeout,
		4: ErrLockNotLocked,
		5: ErrLockDoesNotExistOrInvalidKey,
	}
	assert := assert.New(t)
	for k, v := range codeMap {
		assert.Equal(v, rpcErrorToError(&pb.Error{Code: pb.ErrorCode(k)}), "error code %d", k)
	}

	err := rpcErrorToError(&pb.Error{Code: 0, Message: "foo error"})
	assert.NotNil(err)
	assert.Equal("foo error", err.Error())

	err = rpcErrorToError(&pb.Error{Code: 22, Message: "other error"})
	assert.NotNil(err)
	assert.Equal("unknown RPC error. code: 22 message: other error", err.Error())

}

func patchRetryDelaySeconds(new int) func() {
	old := retryDelaySeconds
	retryDelaySeconds = new
	return func() {
		retryDelaySeconds = old
	}
}

func patchMinRefreshSeconds(new uint32) func() {
	old := minRefreshSeconds
	minRefreshSeconds = new
	return func() {
		minRefreshSeconds = old
	}
}

// implement Closer interface
type closer struct {
	called bool
}

func (c *closer) Close() error { c.called = true; return nil }

func newTestClient(conf *Config, gClient *testGrpcClient) *client {

	return &client{
		conn:          &closer{},
		pbc:           gClient,
		ctx:           context.Background(),
		refreshMap:    sync.Map{},
		noAutoRefresh: conf.NoAutoRefresh,
	}
}

func newTestGrpcClient(lr []*pb.LockResponse, tr []*pb.LockResponse, rr []*pb.LockResponse, ur []*pb.UnlockResponse) *testGrpcClient {
	if lr == nil {
		lr = []*pb.LockResponse{}
	}

	if tr == nil {
		tr = []*pb.LockResponse{}
	}

	if rr == nil {
		rr = []*pb.LockResponse{}
	}

	if ur == nil {
		ur = []*pb.UnlockResponse{}
	}
	return &testGrpcClient{
		lockResponses:        lr,
		tryLockResponses:     tr,
		refreshLockResponses: rr,
		unlockResponses:      ur,
		lockRequests:         make([]*pb.LockRequest, 0),
		tryLockRequests:      make([]*pb.TryLockRequest, 0),
		refreshLockRequests:  make([]*pb.RefreshLockRequest, 0),
		unlockRequests:       make([]*pb.UnlockRequest, 0),
	}
}

// implement pb.LDLMClient interface
type testGrpcClient struct {
	pb.LDLMClient
	lockRequests        []*pb.LockRequest
	tryLockRequests     []*pb.TryLockRequest
	refreshLockRequests []*pb.RefreshLockRequest
	unlockRequests      []*pb.UnlockRequest

	lockResponses        []*pb.LockResponse
	tryLockResponses     []*pb.LockResponse
	refreshLockResponses []*pb.LockResponse
	unlockResponses      []*pb.UnlockResponse
}

func (t *testGrpcClient) Lock(ctx context.Context, in *pb.LockRequest, opts ...grpc.CallOption) (*pb.LockResponse, error) {
	t.lockRequests = append(t.lockRequests, in)

	if len(t.lockResponses) == 0 {
		return &pb.LockResponse{Name: in.Name, Key: uuid.NewString(), Locked: true}, nil
	}

	resp := t.lockResponses[0]
	resp.Name = in.Name
	t.lockResponses = t.lockResponses[1:]
	return resp, nil

}

func (t *testGrpcClient) TryLock(ctx context.Context, in *pb.TryLockRequest, opts ...grpc.CallOption) (*pb.LockResponse, error) {
	t.tryLockRequests = append(t.tryLockRequests, in)

	if len(t.tryLockResponses) == 0 {
		return &pb.LockResponse{Name: in.Name, Key: uuid.NewString(), Locked: true}, nil
	}
	resp := t.tryLockResponses[0]
	resp.Name = in.Name
	t.tryLockResponses = t.tryLockResponses[1:]

	return resp, nil
}

func (t *testGrpcClient) RefreshLock(ctx context.Context, in *pb.RefreshLockRequest, opts ...grpc.CallOption) (*pb.LockResponse, error) {
	t.refreshLockRequests = append(t.refreshLockRequests, in)

	if len(t.refreshLockResponses) == 0 {
		return &pb.LockResponse{Name: in.Name, Key: in.Key, Locked: true}, nil
	}
	resp := t.refreshLockResponses[0]
	resp.Name = in.Name
	resp.Key = in.Key
	t.refreshLockResponses = t.refreshLockResponses[1:]

	return resp, nil
}

func (t *testGrpcClient) Unlock(ctx context.Context, in *pb.UnlockRequest, opts ...grpc.CallOption) (*pb.UnlockResponse, error) {
	t.unlockRequests = append(t.unlockRequests, in)

	if len(t.unlockResponses) == 0 {
		return &pb.UnlockResponse{Unlocked: true}, nil
	}

	resp := t.unlockResponses[0]
	resp.Name = in.Name
	t.unlockResponses = t.unlockResponses[1:]

	return resp, nil
}
