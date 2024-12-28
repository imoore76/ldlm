// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package security_test

import (
	"testing"

	config "github.com/imoore76/configurature"
	"github.com/stretchr/testify/assert"

	con "github.com/imoore76/ldlm/constants"
	"github.com/imoore76/ldlm/net/security"
)

func TestRun_UseTLSConfig(t *testing.T) {
	cases := map[string]struct {
		args     []string
		expected bool
		err      string
	}{
		"cert_and_key": {
			args: []string{
				"--tls_cert", "../../testcerts/server_cert.pem",
				"--tls_key", "../../testcerts/server_key.pem",
			},
			expected: true,
		},
		"just_key": {
			args: []string{
				"--tls_key", "../../testcerts/server_key.pem",
			},
			expected: false,
		},
		"client_cert_verify": {
			args: []string{
				"--client_cert_verify",
			},
			expected: false,
			err:      "client TLS certificate verification requires server TLS to be configured",
		},
		"client_ca": {
			args: []string{
				"--client_ca", "../../testcerts/ca_cert.pem",
			},
			expected: false,
			err:      "client TLS certificate verification requires server TLS to be configured",
		},
		"none": {
			args:     []string{},
			expected: false,
		},
	}

	for name, c := range cases {
		conf := config.Configure[security.SecurityConfig](
			&config.Options{EnvPrefix: con.TestConfigEnvPrefix, Args: c.args},
		)

		tlsConf, err := security.GetTLSConfig(conf)
		if c.err != "" {
			assert.EqualError(t, err, c.err, "Test case %s failed", name)
			continue
		} else {
			assert.Nil(t, err, "Test case %s failed", name)
		}
		assert.True(t, (tlsConf != nil) == c.expected, "Test case %s failed", name)
	}
}

func TestRun_TLSOptions(t *testing.T) {
	assert := assert.New(t)

	conf := config.Configure[security.SecurityConfig](
		&config.Options{EnvPrefix: con.TestConfigEnvPrefix},
	)
	conf.TlsCert = "../../testcerts/server_cert.pem"
	conf.TlsKey = "../../testcerts/server_key.pem"
	conf.ClientCA = "../../testcerts/client_ca_cert.pem"

	tlsConf, err := security.GetTLSConfig(conf)

	assert.NoError(err)
	assert.NotNil(tlsConf)
}
