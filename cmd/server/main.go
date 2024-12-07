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
main() is a simple wrapper around running server.Run()
*/
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	config "github.com/imoore76/configurature"
	"github.com/imoore76/go-ldlm/constants"
	"github.com/imoore76/go-ldlm/log"
	"github.com/imoore76/go-ldlm/net"
	"github.com/imoore76/go-ldlm/server"
)

// Configurature struct for app configuration
type Config struct {
	ConfigFile config.ConfigFile `desc:"Path to yaml configuration file" default:"" short:"c"`
	Version    bool              `desc:"Show version and exit" default:"false"`
	constants.LogLevelConfig
	server.LockServerConfig
	net.NetConfig
}

// Build info populated by goreleaser
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {

	conf := config.Configure[Config](&config.Options{
		EnvPrefix: constants.ConfigEnvPrefix,
		Args:      os.Args[1:],
	})

	if conf.Version {
		fmt.Printf("Version: %s\nCommit: %s\nBuilt: %s\n", version, commit, date)
		os.Exit(0)
	}

	log.SetLevel(conf.LogLevel)

	lockSrv, lockSrvCloser, err := server.New(&conf.LockServerConfig)
	if err != nil {
		os.Stderr.Write([]byte(fmt.Sprintf("server.New() error creating server: %s\n", err)))
		os.Exit(1)
	}

	if netCloser, err := net.Run(lockSrv, &conf.NetConfig); err != nil {
		lockSrvCloser()
		os.Stderr.Write([]byte(fmt.Sprintf("net.Run() error starting server: %s\n", err)))
		os.Exit(1)
	} else {

		// Wait for SIGINT, SIGQUIT, or SIGTERM and exit on receiving any of them
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

		<-sigchan

		netCloser()
		lockSrvCloser()
	}

}
