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
	"fmt"
	"net/rpc"

	"github.com/imoore76/go-ldlm/server/ipc"
)

// Kong struct defining arguments and flags for unlock command
type UnlockArgsAndFlags struct {
	LockName string `arg:"" help:"Lock name"`
	Key      string `arg:"" optional:"" help:"Lock key"`
}

// Run the unlock IPC command
func (cmd *UnlockArgsAndFlags) Run(c *rpc.Client) error {
	unlocked := new(ipc.UnlockResponse)

	// build the reques
	req := ipc.UnlockRequest{
		Name: cmd.LockName,
	}
	if cmd.Key != "" {
		req.Key = cmd.Key
	}

	// call the RPC server over the socket
	err := c.Call("IPC.Unlock", req, unlocked)
	if err != nil {
		return err
	}
	fmt.Println("Unlocked:", *unlocked)
	return nil
}
