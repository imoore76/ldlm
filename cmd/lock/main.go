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

/*
main() is a simple wrapper around making ldlm IPC calls to manipulate locks in
a running ldlm server
*/
package main

import (
	"net/rpc"
	"os"

	"github.com/alecthomas/kong"

	co "github.com/imoore76/go-ldlm/constants"
	"github.com/imoore76/go-ldlm/server/ipc"
)

// newClient creates a new ldlm IPC client.
//
// Takes the socket file path as parameter and returns a pointer to rpc.Client.
func newClient(s string) *rpc.Client {
	if s == "" {
		s = getDefaultSocketPath()
	}
	client, err := rpc.DialHTTP("unix", s)
	if err != nil {
		os.Stderr.Write([]byte("IPC error: " + err.Error() + ". Is the ldlm server running? " +
			"Do you need to specify the socket file path? See help\n"))
		os.Exit(1)
	}
	return client
}

// Struct representing the CLI commands and flags for kong
var cli struct {
	Socket string             `short:"s" help:"Path to the IPC socket file used for communication with the ldlm server" type:"existingfile"`
	Unlock UnlockArgsAndFlags `cmd:"" help:"Unlock a lock"`
	List   ListArgsAndFlags   `cmd:"" help:"List locks"`
}

func main() {
	ctx := kong.Parse(&cli)
	client := newClient(cli.Socket)
	if err := ctx.Run(client); err != nil {
		os.Stderr.Write([]byte(err.Error() + "\n"))
		os.Exit(1)
	}
}

// getDefaultSocketPath makes an educated guess about what socket path to use
// and returns it
func getDefaultSocketPath() string {
	if f := os.Getenv(co.ConfigEnvPrefix + "IPC_SOCKET_FILE"); f != "" {
		return f
	}
	return ipc.DefaultSocketPath()
}
