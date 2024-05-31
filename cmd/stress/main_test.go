// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelp(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm", "stresstest", "-help"}
		main()
		os.Exit(0)
	}

	assert := assert.New(t)

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), "TEST_PASSTHROUGH=1")
	out, err := cmd.Output()
	assert.Nil(err)
	assert.True(strings.Contains(string(out), "--num_clients "), "usage string should be in output")

}

func TestBadAddress(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Args = []string{"ldlm", "--num_clients", "1", "-a", "-"}
		main()
		return
	}

	assert := assert.New(t)

	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), "TEST_PASSTHROUGH=1")
	_, err := cmd.Output()
	assert.NotNil(err)
	e, ok := err.(*exec.ExitError)

	assert.True(ok, "Error should be of type *exec.ExitError")
	assert.False(e.Success(), "Command should have exited with bad error code")
	assert.True(
		strings.Contains(string(e.Stderr), "rpc error: code = Unavailable desc = name resolver error"),
		string(e.Stderr),
	)

}
