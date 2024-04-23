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

package constants

import "log/slog"

const (
	// ConfigEnvPrefix is the prefix used for config environment variables
	ConfigEnvPrefix = "LDLM_"
	// TestConfigEnvPrefix is the prefix used for config env vars for tests
	TestConfigEnvPrefix = "TEST_LDLM_"
)

// Maintain a consistent log level flag across the codebase
type LogLevelConfig struct {
	LogLevel slog.Level `default:"info" short:"v" enum:"debug,info,warn,error" desc:"Log level (debug|info|warn|error)"`
}
