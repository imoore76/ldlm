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

	"github.com/imoore76/go-ldlm/config"
	"github.com/imoore76/go-ldlm/constants"
	"github.com/imoore76/go-ldlm/log"
	"github.com/imoore76/go-ldlm/server"
)

// Configurature struct for app configuration
type Config struct {
	ConfigFile config.File `desc:"Path to yaml configuration file" default:"" short:"c"`
	constants.LogLevelConfig
	server.ServerConfig
}

func main() {

	conf := config.Configure[Config](&config.Options{
		EnvPrefix: constants.ConfigEnvPrefix,
		Args:      os.Args[1:],
	})

	log.SetLevel(conf.LogLevel)

	if stopper, err := server.Run(&conf.ServerConfig); err != nil {
		os.Stderr.Write([]byte(fmt.Sprintf("server.RunServer() error starting server: %s\n", err)))
		os.Exit(1)
	} else {

		// Wait for SIGINT, SIGQUIT, or SIGTERM and exit on receiving any of them
		sigchan := make(chan os.Signal, 1)
		signal.Notify(sigchan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

		<-sigchan

		stopper()
	}

}
