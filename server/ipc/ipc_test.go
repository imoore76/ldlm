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
	"errors"
	"net/rpc"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	cl "github.com/imoore76/go-ldlm/server/clientlock"
	"github.com/imoore76/go-ldlm/server/ipc"
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
	unlockResponse    bool
	unlockError       error
	unlockRequestName string
	getlocksResponse  []cl.Lock
}

func (t *testLockServer) Locks() []cl.Lock {
	return t.getlocksResponse
}

func (t *testLockServer) Unlock(ctx context.Context, name string, key string) (bool, error) {
	t.unlockRequestName = name
	return t.unlockResponse, t.unlockError
}

func newTestLockServer(u bool, l []cl.Lock) *testLockServer {
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

	close, err := ipc.Run(newTestLockServer(false, nil), c)
	defer close()
	assert.NoError(err)
}

func TestUnlock(t *testing.T) {
	assert := assert.New(t)

	ls := newTestLockServer(true, []cl.Lock{
		cl.New("mylock", "bar", 1),
	})

	c, cleanup := conf()
	defer cleanup()

	close, err := ipc.Run(ls, c)
	defer close()
	assert.NoError(err)

	client := newClient(c)

	unlocked := new(ipc.UnlockResponse)
	err = client.Call("IPC.Unlock", ipc.UnlockRequest{Name: "mylock"}, unlocked)
	assert.Nil(err)
	assert.True(bool(*unlocked))
}

func TestUnlockWithKey(t *testing.T) {
	assert := assert.New(t)

	ls := newTestLockServer(true, []cl.Lock{})

	c, cleanup := conf()
	defer cleanup()

	close, err := ipc.Run(ls, c)
	defer close()
	assert.NoError(err)

	client := newClient(c)

	unlocked := new(ipc.UnlockResponse)
	err = client.Call("IPC.Unlock", ipc.UnlockRequest{Name: "mylock", Key: "asdf"}, unlocked)
	assert.Nil(err)
	assert.True(bool(*unlocked))
}

func TestUnlockFail(t *testing.T) {
	assert := assert.New(t)

	ls := newTestLockServer(false, []cl.Lock{
		cl.New("mylock", "bar", 1),
	})
	ls.unlockError = errors.New("things and stuff went wrong")

	c, cleanup := conf()
	defer cleanup()

	close, err := ipc.Run(ls, c)
	assert.NoError(err)
	defer close()

	client := newClient(c)

	unlocked := new(ipc.UnlockResponse)
	err = client.Call("IPC.Unlock", ipc.UnlockRequest{Name: "mylock"}, unlocked)
	assert.Error(err)
	assert.False(bool(*unlocked))
	assert.True(strings.Contains(err.Error(), "things and stuff went wrong"), "Unexpected error: %v", err)
}

func TestUnlock_LockDoesNotExist(t *testing.T) {
	assert := assert.New(t)

	c, cleanup := conf()
	defer cleanup()

	close, err := ipc.Run(newTestLockServer(false, []cl.Lock{
		cl.New("foo", "bar", 1),
	}), c)
	defer close()
	assert.NoError(err)

	client := newClient(c)

	unlocked := new(ipc.UnlockResponse)
	err = client.Call("IPC.Unlock", ipc.UnlockRequest{Name: "mylock"}, unlocked)
	assert.Error(err)
	assert.False(bool(*unlocked))

	assert.True(strings.Contains(err.Error(), "a lock by that name does not exist"))
}

func TestListLocks(t *testing.T) {
	assert := assert.New(t)

	locks := []cl.Lock{
		cl.New("foo", "bar", 22),
		cl.New("you", "there", 2),
		cl.New("a", "baz", 1),
		cl.New("b", "baz", 1),
		cl.New("c", "baz", 4),
		cl.New("you", "here", 6),
	}

	c, cleanup := conf()
	defer cleanup()

	close, err := ipc.Run(newTestLockServer(false, locks), c)
	assert.NoError(err)
	defer close()
	client := newClient(c)

	lockList := new(ipc.ListLocksResponse)
	err = client.Call("IPC.ListLocks", ipc.ListLocksRequest{}, &lockList)
	assert.Nil(err)

	assert.Equal(
		"{Name: foo, Key: bar, Size: 22},"+
			"{Name: you, Key: there, Size: 2},"+
			"{Name: a, Key: baz, Size: 1},"+
			"{Name: b, Key: baz, Size: 1},"+
			"{Name: c, Key: baz, Size: 4},"+
			"{Name: you, Key: here, Size: 6}",
		strings.Join([]string(*lockList), ","),
	)
}

func TestListLocks_NoLocks(t *testing.T) {
	assert := assert.New(t)

	locks := []cl.Lock{}

	c, cleanup := conf()
	defer cleanup()

	close, err := ipc.Run(newTestLockServer(false, locks), c)
	assert.NoError(err)
	defer close()

	client := newClient(c)

	lockList := new(ipc.ListLocksResponse)
	err = client.Call("IPC.ListLocks", ipc.ListLocksRequest{}, &lockList)
	assert.Nil(err)
	assert.Len(*lockList, 0)
}
