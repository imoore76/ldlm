// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var ipcSocketPath string

func init() {
	if tmpfile, err := os.CreateTemp(os.TempDir(), "ldlm-test-ipc-*"); err != nil {
		panic(err)
	} else {
		tmpfile.Close()
		ipcSocketPath = tmpfile.Name()
		os.Remove(ipcSocketPath)
	}
}

func runMain(t *testing.T) (out string, err string) {
	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), "LDLM_SOCKET_FILE="+ipcSocketPath, "TEST_PASSTHROUGH=1")
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

func TestServerHelp(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm", "-help"}
		main()
		os.Exit(0)
	}

	assert := assert.New(t)

	stdout, stderr := runMain(t)
	assert.Equal("", stderr)
	assert.True(
		strings.Contains(stdout, "--default_lock_timeout duration"),
		fmt.Sprintf("usage string should be in output: %s", stdout),
	)
}

func TestServerError(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm", "-l", "b.1.2.3:x"}
		main()
		os.Exit(0)
	}

	assert := assert.New(t)

	_, stderr := runMain(t)

	assert.True(strings.Contains(stderr, "server.RunServer() error starting server: listen tcp: lookup tcp/x: unknown port"),
		fmt.Sprintf("error string should be in output: %s", stderr))

}
