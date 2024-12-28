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
	"net/rpc"

	"github.com/imoore76/ldlm/server/ipc"
)

// Kong struct defining arguments and flags for list command
type ListArgsAndFlags struct{}

// Run the list IPC command
func (*ListArgsAndFlags) Run(c *rpc.Client) error {
	lockList := new(ipc.ListLocksResponse)

	// Call the RPC server over the socket
	err := c.Call("IPC.ListLocks", ipc.ListLocksRequest{}, &lockList)
	if err != nil {
		return err
	}
	// Print the list of locks
	if len(*lockList) == 0 {
		fmt.Println("No locks found")
		return nil
	}
	for _, l := range *lockList {
		fmt.Println(l)
	}
	return nil
}
