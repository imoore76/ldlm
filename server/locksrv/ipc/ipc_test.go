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

package ipc_test

import (
	"context"
	"net/rpc"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/imoore76/go-ldlm/protos"
	"github.com/imoore76/go-ldlm/server/locksrv/ipc"
	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
)

func conf() (*ipc.IPCConfig, func()) {
	tmpfl, _ := os.CreateTemp("", "ipc-test-*.sock")
	tmpfl.Close()
	os.Remove(tmpfl.Name())

	return &ipc.IPCConfig{
			IPCSocketFile: tmpfl.Name(),
		}, func() {
			os.Remove(tmpfl.Name())
		}
}

type testLockServer struct {
	unlockResponse   *pb.UnlockResponse
	unlockRequest    *pb.UnlockRequest
	getlocksResponse []cl.ClientLock
}

func (t *testLockServer) Locks() []cl.ClientLock {
	return t.getlocksResponse
}

func (t *testLockServer) Unlock(ctx context.Context, req *pb.UnlockRequest) (*pb.UnlockResponse, error) {
	t.unlockRequest = req
	return t.unlockResponse, nil
}

func newTestLockServer(u *pb.UnlockResponse, l []cl.ClientLock) *testLockServer {
	return &testLockServer{
		unlockResponse:   u,
		getlocksResponse: l,
	}
}

func newClient(conf *ipc.IPCConfig) *rpc.Client {

	client, err := rpc.DialHTTP("unix", conf.IPCSocketFile)
	if err != nil {
		panic(err)
	}
	return client
}

func TestRun(t *testing.T) {
	assert := assert.New(t)
	c, cleanup := conf()
	defer cleanup()
	cancel, err := ipc.Run(newTestLockServer(nil, nil), c)
	defer cancel()
	assert.NoError(err)
}

func TestUnlock(t *testing.T) {
	assert := assert.New(t)

	resp := &pb.UnlockResponse{
		Unlocked: true,
		Name:     "mylock",
	}
	ls := newTestLockServer(resp, []cl.ClientLock{
		*cl.New("mylock", "bar"),
	})

	c, cleanup := conf()
	defer cleanup()

	cancel, err := ipc.Run(ls, c)
	assert.NoError(err)
	defer cancel()

	client := newClient(c)

	unlocked := new(ipc.UnlockResponse)
	err = client.Call("IPC.Unlock", ipc.UnlockRequest{Name: "mylock"}, unlocked)
	assert.Nil(err)
	assert.True(bool(*unlocked))
}

func TestUnlockWithKey(t *testing.T) {
	assert := assert.New(t)

	resp := &pb.UnlockResponse{
		Unlocked: true,
		Name:     "mylock",
	}
	ls := newTestLockServer(resp, []cl.ClientLock{})

	c, cleanup := conf()
	defer cleanup()

	cancel, err := ipc.Run(ls, c)
	assert.NoError(err)
	defer cancel()

	client := newClient(c)

	unlocked := new(ipc.UnlockResponse)
	err = client.Call("IPC.Unlock", ipc.UnlockRequest{Name: "mylock", Key: "asdf"}, unlocked)
	assert.Nil(err)
	assert.True(bool(*unlocked))
}

func TestUnlockFail(t *testing.T) {
	assert := assert.New(t)

	resp := &pb.UnlockResponse{
		Unlocked: false,
		Name:     "mylock",
		Error: &pb.Error{
			Code:    pb.ErrorCode_Unknown,
			Message: "things and stuff went wrong",
		},
	}
	ls := newTestLockServer(resp, []cl.ClientLock{
		*cl.New("mylock", "bar"),
	})

	c, cleanup := conf()
	defer cleanup()

	cancel, err := ipc.Run(ls, c)
	assert.NoError(err)
	defer cancel()

	client := newClient(c)

	unlocked := new(ipc.UnlockResponse)
	err = client.Call("IPC.Unlock", ipc.UnlockRequest{Name: "mylock"}, unlocked)
	assert.Error(err)
	assert.False(bool(*unlocked))
	assert.True(strings.Contains(err.Error(), "failed to unlock lock"), "Unexpected error: %v", err)
}

func TestUnlock_LockDoesNotExist(t *testing.T) {
	assert := assert.New(t)

	c, cleanup := conf()
	defer cleanup()

	cancel, err := ipc.Run(newTestLockServer(nil, []cl.ClientLock{
		*cl.New("foo", "bar"),
	}), c)
	assert.NoError(err)
	defer cancel()

	client := newClient(c)

	unlocked := new(ipc.UnlockResponse)
	err = client.Call("IPC.Unlock", ipc.UnlockRequest{Name: "mylock"}, unlocked)
	assert.Error(err)
	assert.False(bool(*unlocked))

	assert.True(strings.Contains(err.Error(), "a lock by that name does not exist"))
}

func TestListLocks(t *testing.T) {
	assert := assert.New(t)

	locks := []cl.ClientLock{
		*cl.New("foo", "bar"),
		*cl.New("you", "there"),
		*cl.New("a", "baz"),
		*cl.New("b", "baz"),
		*cl.New("c", "baz"),
		*cl.New("you", "here"),
	}

	c, cleanup := conf()
	defer cleanup()

	cancel, err := ipc.Run(newTestLockServer(nil, locks), c)
	assert.NoError(err)
	defer cancel()
	client := newClient(c)

	lockList := new(ipc.ListLocksResponse)
	err = client.Call("IPC.ListLocks", ipc.ListLocksRequest{}, &lockList)
	assert.Nil(err)

	assert.Equal(
		"{Name: foo, Key: bar},{Name: you, Key: there},{Name: a, Key: baz},"+
			"{Name: b, Key: baz},{Name: c, Key: baz},{Name: you, Key: here}",
		strings.Join([]string(*lockList), ","),
	)
}

func TestListLocks_NoLocks(t *testing.T) {
	assert := assert.New(t)

	locks := []cl.ClientLock{}

	c, cleanup := conf()
	defer cleanup()

	cancel, err := ipc.Run(newTestLockServer(nil, locks), c)
	assert.NoError(err)
	defer cancel()
	client := newClient(c)

	lockList := new(ipc.ListLocksResponse)
	err = client.Call("IPC.ListLocks", ipc.ListLocksRequest{}, &lockList)
	assert.Nil(err)
	assert.Len(*lockList, 0)
}
