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
main() is a simple wrapper around running stresstest.Run()
*/
package main

import (
	"os"

	cnf "github.com/imoore76/configurature"
	"github.com/imoore76/ldlm/constants"
	"github.com/imoore76/ldlm/stresstest"

	"github.com/imoore76/ldlm/log"
)

// Config for the stress test
type Config struct {
	stresstest.Config
	constants.LogLevelConfig
}

func main() {

	c := cnf.Configure[Config](&cnf.Options{
		EnvPrefix: constants.ConfigEnvPrefix,
		Args:      os.Args[1:],
	})

	log.SetLevel(c.LogLevel)
	stresstest.Run(&c.Config)

}
