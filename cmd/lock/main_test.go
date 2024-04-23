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

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	co "github.com/imoore76/go-ldlm/constants"
	pb "github.com/imoore76/go-ldlm/protos"
	"github.com/imoore76/go-ldlm/server/locksrv/ipc"
	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
)

var testSocketPath string
var conf *ipc.IPCConfig

func init() {
	if testSocketPath = os.Getenv(co.TestConfigEnvPrefix + "IPC_SOCKET_FILE"); testSocketPath == "" {
		if tmpfile, err := os.CreateTemp(os.TempDir(), "ldlm-test-ipc-*"); err != nil {
			panic(err)
		} else {
			tmpfile.Close()
			testSocketPath = tmpfile.Name()
			os.Remove(testSocketPath)
		}
	}
	conf = &ipc.IPCConfig{
		IPCSocketFile: testSocketPath,
	}
}

// Helper function to run main() and return stdout and stderr
func runMain(t *testing.T) (out string, err string) {
	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), co.ConfigEnvPrefix+"IPC_SOCKET_FILE="+testSocketPath, "TEST_PASSTHROUGH=1")
	cmdout, cmderr := cmd.Output()
	if cmdout != nil {
		out = string(cmdout)
	} else {
		out = ""
	}
	if e, ok := cmderr.(*exec.ExitError); ok {
		err = string(e.Stderr)
	} else {
		err = ""
	}
	return out, err
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

func TestUsage(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm"}
		main()
		os.Exit(0)
	}

	assert := assert.New(t)
	cancel, err := ipc.Run(newTestLockServer(nil, nil), conf)
	defer cancel()
	assert.Nil(err)

	out, errout := runMain(t)
	assert.Equal("", out)
	assert.Equal("ldlm: error: expected one of \"unlock\",  \"list\"\n", errout)
}

func TestUnknownCommand(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm", "foo"}
		main()
		os.Exit(0)
	}

	assert := assert.New(t)
	cancel, err := ipc.Run(newTestLockServer(nil, nil), conf)
	defer cancel()
	assert.Nil(err)

	out, errout := runMain(t)
	assert.Equal("", out)
	assert.True(strings.Contains(errout, "unexpected argument foo"), errout)
}

func TestUnlock(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm-lock", "unlock", "mylock2"}
		main()
		os.Exit(0)
	}

	assert := assert.New(t)

	resp := &pb.UnlockResponse{
		Unlocked: true,
		Name:     "mylock2",
	}
	ls := newTestLockServer(resp, []cl.ClientLock{
		*cl.New("mylock2", "bar"),
	})
	cancel, err := ipc.Run(ls, conf)
	fmt.Printf("conf: %v\n", conf)
	assert.NoError(err)
	defer cancel()

	out, errout := runMain(t)

	assert.Equal("Unlocked: true\n", out)
	assert.Equal("", errout)
}

func TestUnlockFail(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm-lock", "unlock", "mylock"}
		main()
		os.Exit(0)
	}

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
	cancel, err := ipc.Run(ls, conf)
	assert.NoError(err)
	defer cancel()

	out, errout := runMain(t)
	assert.Equal("failed to unlock lock: things and stuff went wrong\n", errout)
	assert.Equal("", out)
}

func TestUnlock_LockDoesNotExist(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm-lock", "unlock", "mylock"}
		main()
		os.Exit(0)
	}

	assert := assert.New(t)

	cancel, err := ipc.Run(newTestLockServer(nil, []cl.ClientLock{
		*cl.New("foo", "bar"),
	}), conf)
	assert.NoError(err)
	defer cancel()

	out, errout := runMain(t)
	assert.Equal("a lock by that name does not exist\n", errout)
	assert.Equal("", out)

}

func TestListLocks(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm", "list"}
		main()
		os.Exit(0)
	}

	assert := assert.New(t)

	locks := []cl.ClientLock{
		*cl.New("foo", "bar"),
		*cl.New("you", "there"),
		*cl.New("a", "baz"),
		*cl.New("b", "baz"),
		*cl.New("c", "baz"),
		*cl.New("you", "here"),
	}

	cancel, err := ipc.Run(newTestLockServer(nil, locks), conf)
	assert.NoError(err)
	defer cancel()

	out, errout := runMain(t)
	assert.Equal(
		"{Name: foo, Key: bar}\n{Name: you, Key: there}\n{Name: a, Key: baz}\n"+
			"{Name: b, Key: baz}\n{Name: c, Key: baz}\n{Name: you, Key: here}\n",
		out)
	assert.Equal("", errout)

}

func TestListLocks_NoLocks(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm", "list"}
		main()
		os.Exit(0)
	}
	assert := assert.New(t)

	locks := []cl.ClientLock{}

	cancel, err := ipc.Run(newTestLockServer(nil, locks), conf)
	assert.NoError(err)
	defer cancel()

	out, errout := runMain(t)
	assert.Equal("No locks found\n", out)
	assert.Equal("", errout)

}
