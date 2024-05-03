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
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	co "github.com/imoore76/go-ldlm/constants"
	_ "github.com/imoore76/go-ldlm/log"
	cl "github.com/imoore76/go-ldlm/server/clientlock"
	"github.com/imoore76/go-ldlm/server/ipc"
)

var testSocketPath string
var conf *ipc.IPCConfig

func setConfig() {
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

func TestUsage(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm"}
		main()
		os.Exit(0)
	}

	setConfig()
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
	setConfig()
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
	setConfig()
	assert := assert.New(t)

	ls := newTestLockServer(true, []cl.Lock{
		cl.New("mylock2", "bar"),
	})

	cancel, err := ipc.Run(ls, conf)
	defer cancel()
	assert.NoError(err)

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
	setConfig()
	assert := assert.New(t)

	resp := errors.New("things and stuff went wrong")
	ls := newTestLockServer(resp, []cl.Lock{
		cl.New("mylock", "bar"),
	})
	cancel, err := ipc.Run(ls, conf)
	defer cancel()
	assert.NoError(err)

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
	setConfig()
	assert := assert.New(t)

	ls := newTestLockServer(nil, []cl.Lock{
		cl.New("foo", "bar"),
	})

	cancel, err := ipc.Run(ls, conf)
	defer cancel()

	assert.NoError(err)

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
	setConfig()
	assert := assert.New(t)

	locks := []cl.Lock{
		cl.New("foo", "bar"),
		cl.New("you", "there"),
		cl.New("a", "baz"),
		cl.New("b", "baz"),
		cl.New("c", "baz"),
		cl.New("you", "here"),
	}

	ls := newTestLockServer(nil, locks)
	cancel, err := ipc.Run(ls, conf)
	defer cancel()

	assert.NoError(err)

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
	setConfig()
	assert := assert.New(t)

	locks := []cl.Lock{}

	ls := newTestLockServer(nil, locks)
	cancel, err := ipc.Run(ls, conf)
	defer cancel()

	assert.NoError(err)

	out, errout := runMain(t)
	assert.Equal("No locks found\n", out)
	assert.Equal("", errout)

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
	unlockResponse interface{}
	unlockRequest  struct {
		name string
		key  string
	}
	getlocksResponse []cl.Lock
}

func (t *testLockServer) Locks() []cl.Lock {
	return t.getlocksResponse
}

func (t *testLockServer) Unlock(ctx context.Context, name string, key string) (bool, error) {
	t.unlockRequest = struct {
		name string
		key  string
	}{
		name: name,
		key:  key,
	}
	switch tp := t.unlockResponse.(type) {
	case error:
		return false, tp
	case bool:
		return tp, nil
	}
	return false, errors.New("Unknown response type")
}

func newTestLockServer(u interface{}, l []cl.Lock) *testLockServer {
	if l == nil {
		l = []cl.Lock{}
	}
	return &testLockServer{
		unlockResponse:   u,
		getlocksResponse: l,
	}
}
