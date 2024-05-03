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

/*
This file contains generic security functions and types for networking components that interact with
a lock server
*/
package security

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// Common security configuration
type SecurityConfig struct {
	TlsCert          string `desc:"File containing TLS certificate" default:""`
	TlsKey           string `desc:"File containing TLS key" default:""`
	ClientCertVerify bool   `desc:"Verify client certificate" default:"false"`
	ClientCA         string `desc:"File containing client CA certificate. This will also enable client cert verification." default:""`
	Password         string `desc:"Password required of clients" default:""`
}

// Return tls.Config if tls is configured, or an error
func GetTLSConfig(conf *SecurityConfig) (*tls.Config, error) {
	useTls := false

	// Create tls config
	tlsConfig := &tls.Config{}

	// Load server's certificate and private key
	if conf.TlsCert != "" {
		serverCert, err := tls.LoadX509KeyPair(conf.TlsCert, conf.TlsKey)
		if err != nil {
			return nil, fmt.Errorf("LoadX509KeyPair() error loading cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{serverCert}
		useTls = true
	}

	// Client certificate verification
	if conf.ClientCA != "" {
		caPem, err := os.ReadFile(conf.ClientCA)
		if err != nil {
			return nil, fmt.Errorf("os.ReadFile() failed to read ca cert: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caPem) {
			return nil, fmt.Errorf("AppendCertsFromPEM() failed to append client ca cert")
		}
		tlsConfig.ClientCAs = certPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		useTls = true

	} else if conf.ClientCertVerify {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		useTls = true
	}

	// Can't do client TLS without server TLS
	if useTls && conf.TlsCert == "" {
		return nil, fmt.Errorf("client TLS certificate verification requires server TLS to be configured")
	}

	if useTls {
		return tlsConfig, nil
	}

	return nil, nil

}
