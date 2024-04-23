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

package config_test

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/stretchr/testify/assert"

	co "github.com/imoore76/go-ldlm/config"
)

type SubConfig struct {
	StateFile           string        `desc:"File in which to store lock state" default:"" short:"s"`
	DefaultLockTimeout  time.Duration `desc:"Lock timeout to use when loading locks from state file on startup" default:"10m" short:"d"`
	NoClearOnDisconnect bool          `desc:"Do not clear locks on client disconnect" default:"false" short:"c"`
	FooSeconds          uint32        `desc:"Something" default:"10" short:"f"`
	FooInt              uint32        `desc:"Something" default:"100" short:"o"`
}

type OtherSubConfig struct {
	SubFooString string `desc:"Something" default:"here"`
}

type TestConfig struct {
	SubConfig
	Bool              bool          `desc:"Bool thing" default:"false"`
	KeepaliveInterval time.Duration `desc:"Interval at which to send keepalive pings to client" default:"60s" short:"k"`
	KeepaliveTimeout  time.Duration `desc:"Wait this duration for the ping ack before assuming the connection is dead" default:"5s" short:"t"`
	LockGcInterval    time.Duration `desc:"Interval at which to garbage collect unused locks." default:"30m" short:"g"`
	OtherSubConfig
	LockGcMinIdle time.Duration `desc:"Minimum time a lock has to be idle (no unlocks or locks) before being considered for garbage collection" default:"5m" short:"m"`
	ListenAddress string        `desc:"Address (host:port) at which to listen" default:"localhost:3144" short:"l"`
	LogLevel      slog.Level    `desc:"Log level (debug|info|warn|error)" default:"info" short:"v"`
}

type TestConfigFileStruct struct {
	CoolFile co.File `desc:"Configuration file" default:""`
	TestConfig
}

func runExternal(t *testing.T) (out string, err string) {
	cmd := exec.Command(os.Args[0], "-test.run="+t.Name())
	cmd.Env = append(os.Environ(), "TEST_PASSTHROUGH=1")
	cmdout, cmderr := cmd.Output()
	if cmdout != nil {
		out = string(cmdout)
	} else {
		out = ""
	}
	if e, ok := cmderr.(*exec.ExitError); ok {
		err = string(e.Stderr)
	} else {
		err = ""
	}
	return out, err
}

func TestDefaults(t *testing.T) {
	c := co.Configure[TestConfig](&co.Options{})

	if c.SubFooString != "here" {
		t.Errorf("Test failed, SubFooString expected: '%s', got: '%s'", "here", c.SubFooString)
	}
	if c.FooInt != 100 {
		t.Errorf("Test failed, FooInt expected: '%d', got: '%d'", 100, c.FooInt)
	}
	if c.FooSeconds != uint32(10) {
		t.Errorf("Test failed, FooSeconds expected: '%d', got: '%d'", 10, c.FooSeconds)
	}
	if c.LogLevel != slog.LevelInfo {
		t.Errorf("Test failed, LogLevel expected: '%s', got: '%s'", "INFO", c.LogLevel)
	}
	if c.KeepaliveInterval != time.Duration(60)*time.Second {
		t.Errorf("Test failed, KeepaliveInterval expected: '%s', got: '%s'", "60s", c.KeepaliveInterval)
	}
	if c.KeepaliveTimeout != time.Duration(5)*time.Second {
		t.Errorf("Test failed, KeepaliveTimeout expected: '%s', got: '%s'", "5s", c.KeepaliveTimeout)
	}
	if c.LockGcInterval != time.Duration(30)*time.Minute {
		t.Errorf("Test failed, LockGcInterval expected: '%s', got: '%s'", "30m", c.LockGcInterval)
	}
	if c.LockGcMinIdle != time.Duration(5)*time.Minute {
		t.Errorf("Test failed, LockGcMinIdle expected: '%s', got: '%s'", "5m", c.LockGcMinIdle)
	}
	if c.ListenAddress != "localhost:3144" {
		t.Errorf("Test failed, ListenAddress expected: '%s', got: '%s'", "localhost:3144", c.ListenAddress)
	}

}
func TestParseFlags(t *testing.T) {
	args := []string{"cmd", "-v", "warn", "-k", "80m", "-t", "333m", "-g",
		"1s", "-m", "29h", "-l", "0.0.0.0:1", "-o", "7", "--sub_foo_string", "yes",
		"--bool"}

	c := co.Configure[TestConfig](&co.Options{Args: args})

	if c.Bool != true {
		t.Errorf("Test failed, Bool expected: 'true', got: '%v'", c.Bool)
	}
	if c.SubFooString != "yes" {
		t.Errorf("Test failed, SubFooString expected: '%s', got: '%s'", "yes", c.SubFooString)
	}

	if c.LogLevel != slog.LevelWarn {
		t.Errorf("Test failed, LogLevel expected: '%s', got:  '%s'", "WARN", c.LogLevel)
	}
	if c.KeepaliveInterval != time.Duration(80)*time.Minute {
		t.Errorf("Test failed, KeepaliveInterval expected: '%s', got:  '%s'", "80m", c.KeepaliveInterval)
	}
	if c.KeepaliveTimeout != time.Duration(333)*time.Minute {
		t.Errorf("Test failed, KeepaliveTimeout expected: '%s', got:  '%s'", "333m", c.KeepaliveTimeout)
	}
	if c.LockGcInterval != time.Duration(1)*time.Second {
		t.Errorf("Test failed, LockGcInterval expected: '%s', got:  '%s'", "1s", c.LockGcInterval)
	}
	if c.LockGcMinIdle != time.Duration(29)*time.Hour {
		t.Errorf("Test failed, LockGcMinIdle expected: '%s', got:  '%s'", "29m", c.LockGcMinIdle)
	}
	if c.ListenAddress != "0.0.0.0:1" {
		t.Errorf("Test failed, ListenAddress expected: '%s', got:  '%s'", "0.0.0.0:1", c.ListenAddress)
	}
	if c.FooInt != 7 {
		t.Errorf("Test failed, FooInt expected: '%d', got:  '%d'", 7, c.FooInt)
	}
}

func TestEnvVars(t *testing.T) {
	origEnviron := os.Environ()
	defer func() {
		for _, e := range origEnviron {
			pair := strings.SplitN(e, "=", 2)
			os.Setenv(pair[0], pair[1])
		}
	}()
	// Clear environ
	for _, e := range origEnviron {
		pair := strings.SplitN(e, "=", 2)
		os.Setenv(pair[0], "")
	}

	envPrefix := "FOO_"
	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("KeepaliveInterval")),
		"7h",
	)
	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("KeepaliveTimeout")),
		"21h",
	)
	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("LockGcInterval")),
		"88h",
	)
	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("LockGcMinIdle")),
		"25h",
	)
	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("ListenAddress")),
		"127.0.0.1:22",
	)
	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("LogLevel")),
		"ERROR",
	)

	c := co.Configure[TestConfig](&co.Options{EnvPrefix: envPrefix})

	if c.LogLevel != slog.LevelError {
		t.Errorf("Test failed, LogLevel expected: '%s', got:  '%s'", "ERROR", c.LogLevel)
	}
	if c.KeepaliveInterval != time.Duration(7)*time.Hour {
		t.Errorf("Test failed, KeepaliveInterval expected: '%s', got:  '%s'", "7h", c.KeepaliveInterval)
	}
	if c.KeepaliveTimeout != time.Duration(21)*time.Hour {
		t.Errorf("Test failed, KeepaliveTimeout expected: '%s', got:  '%s'", "21h", c.KeepaliveTimeout)
	}
	if c.LockGcInterval != time.Duration(88)*time.Hour {
		t.Errorf("Test failed, LockGcInterval expected: '%s', got:  '%s'", "88h", c.LockGcInterval)
	}
	if c.LockGcMinIdle != time.Duration(25)*time.Hour {
		t.Errorf("Test failed, LockGcMinIdle expected: '%s', got:  '%s'", "25h", c.LockGcMinIdle)
	}
	if c.ListenAddress != "127.0.0.1:22" {
		t.Errorf("Test failed, ListenAddress expected: '%s', got:  '%s'", "127.0.0.1:22", c.ListenAddress)
	}
}

func TestFlagOverrideEnv(t *testing.T) {
	/*
		test all Type()s of config values
	*/
	envPrefix := "BAR_"
	origEnviron := os.Environ()
	defer func() {
		for _, e := range origEnviron {
			pair := strings.SplitN(e, "=", 2)
			os.Setenv(pair[0], pair[1])
		}
	}()
	// Clear environ
	for _, e := range origEnviron {
		pair := strings.SplitN(e, "=", 2)
		os.Setenv(pair[0], "")
	}

	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("KeepaliveInterval")),
		"7h",
	)
	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("ListenAddress")),
		"127.0.0.1:22",
	)
	os.Setenv(
		fmt.Sprintf("%s%s", envPrefix, strcase.ToScreamingSnake("LogLevel")),
		"ERROR",
	)

	args := []string{"-k", "88m", "-v", "warn", "-l", "0.0.0.0:443"}
	c := co.Configure[TestConfig](&co.Options{EnvPrefix: envPrefix, Args: args})

	if c.LogLevel != slog.LevelWarn {
		t.Errorf("Test failed, LogLevel expected: '%s', got:  '%s'", "WARN", c.LogLevel)
	}
	if c.KeepaliveInterval != time.Duration(88)*time.Minute {
		t.Errorf("Test failed, KeepaliveInterval expected: '%s', got:  '%s'", "88m", c.KeepaliveInterval)
	}
	if c.ListenAddress != "0.0.0.0:443" {
		t.Errorf("Test failed, ListenAddress expected: '%s', got:  '%s'", "0.0.0.0:443", c.ListenAddress)
	}

}

func TestConfigFile(t *testing.T) {
	assert := assert.New(t)

	tmp, _ := os.CreateTemp("", "ldlm-test-*.yml")
	defer os.Remove(tmp.Name())
	tmp.Write([]byte("foo_int: 4\nsub_foo_string: 'yes'\nkeepalive_timeout: 3m\nbool: true\n"))
	tmp.Close()

	c := co.Configure[TestConfigFileStruct](&co.Options{
		Args: []string{"--cool_file", tmp.Name()},
	})
	assert.Equal(uint32(4), c.FooInt)
	assert.Equal("yes", c.SubFooString)
	assert.Equal(time.Duration(3)*time.Minute, c.KeepaliveTimeout)
	assert.Equal(true, c.Bool)
}

func TestConfigFile_FromEnv(t *testing.T) {
	assert := assert.New(t)

	tmp, _ := os.CreateTemp("", "ldlm-test-*.yml")
	defer os.Remove(tmp.Name())
	tmp.Write([]byte("foo_int: 4\nsub_foo_string: 'yes'\nkeepalive_timeout: 3m\nbool: true\n"))
	tmp.Close()

	os.Setenv("TEST_CONF_COOL_FILE", tmp.Name())
	defer os.Setenv("TEST_CONF_COOL_FILE", "")

	c := co.Configure[TestConfigFileStruct](&co.Options{
		EnvPrefix: "TEST_CONF_",
	})

	assert.Equal(uint32(4), c.FooInt)
	assert.Equal("yes", c.SubFooString)
	assert.Equal(time.Duration(3)*time.Minute, c.KeepaliveTimeout)
	assert.Equal(true, c.Bool)
}

func TestConfigFile_Precedence(t *testing.T) {
	assert := assert.New(t)

	os.Setenv("TEST_CONF_FOO_INT", "7")
	os.Setenv("TEST_CONF_SUB_FOO_STRING", "asdf")

	tmp, _ := os.CreateTemp("", "ldlm-test-*.yml")
	defer os.Remove(tmp.Name())
	tmp.Write([]byte("foo_int: 4\nsub_foo_string: 'yes'\nkeepalive_timeout: 3m\nbool: false\n"))
	tmp.Close()

	c := co.Configure[TestConfigFileStruct](&co.Options{
		EnvPrefix: "TEST_CONF_",
		Args:      []string{"--cool_file", tmp.Name(), "--foo_int", "22"},
	})
	assert.Equal(uint32(22), c.FooInt)
	assert.Equal("asdf", c.SubFooString)
	assert.Equal(time.Duration(3)*time.Minute, c.KeepaliveTimeout)
	assert.Equal(false, c.Bool)

}

func TestBadEnvVar(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		os.Setenv("TEST_CONF_FOO_INT", "asdf")
		co.Configure[TestConfigFileStruct](&co.Options{
			EnvPrefix: "TEST_CONF_",
		})
		os.Exit(0)
	}

	assert := assert.New(t)
	stdout, stderr := runExternal(t)

	assert.Equal("", stderr)
	assert.Equal("error parsing configuration: setFromEnv(): unable to parse uint32 value: asdf: "+
		"strconv.ParseUint: parsing \"asdf\": invalid syntax\n", stdout)
}

func TestBadFlagValue(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		co.Configure[TestConfigFileStruct](&co.Options{
			Args: []string{"-o", "asdf"},
		})
		os.Exit(0)
	}

	assert := assert.New(t)
	stdout, stderr := runExternal(t)

	assert.Equal("", stderr)
	assert.True(strings.HasPrefix(stdout, "Command usage:"))
}

func TestBadFlag(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		co.Configure[TestConfigFileStruct](&co.Options{
			Args: []string{"--thing_here", "asdf"},
		})
		os.Exit(0)
	}

	assert := assert.New(t)
	stdout, stderr := runExternal(t)

	assert.Equal("", stderr)
	assert.True(strings.HasPrefix(stdout, "Command usage:"))
}

func TestConfigFile_BadField(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		tmp, _ := os.CreateTemp("", "ldlm-test-*.yml")
		defer os.Remove(tmp.Name())
		tmp.Write([]byte("foo_int: 4\nsub_string: 'yes'\nkeepalive_timeout: 3m\nbool: true\n"))
		tmp.Close()

		co.Configure[TestConfigFileStruct](&co.Options{
			Args: []string{"--cool_file", tmp.Name()},
		})
		os.Exit(0)
	}
	assert := assert.New(t)
	stdout, stderr := runExternal(t)

	assert.Equal("", stderr)
	assert.Equal("error parsing configuration: unknown configuration file field: sub_string\n", stdout)

}

func TestConfigFile_BadValue(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		tmp, _ := os.CreateTemp("", "ldlm-test-*.yml")
		defer os.Remove(tmp.Name())
		tmp.Write([]byte("foo_int: asdf\nsub_foo_string: 'ok'\nkeepalive_timeout: 3m\nbool: true\n"))
		tmp.Close()

		co.Configure[TestConfigFileStruct](&co.Options{
			Args: []string{"--cool_file", tmp.Name()},
		})
		os.Exit(0)
	}
	assert := assert.New(t)
	stdout, stderr := runExternal(t)

	assert.Equal("", stderr)
	assert.Equal("error parsing configuration: error parsing config file value: unable to parse "+
		"uint32 value: asdf: strconv.ParseUint: parsing \"asdf\": invalid syntax\n", stdout)

}

func TestConfigFile_UnsupportedType(t *testing.T) {
	if os.Getenv("TEST_PASSTHROUGH") == "1" {
		tmp, _ := os.CreateTemp("", "ldlm-test-*.foo")
		defer os.Remove(tmp.Name())
		tmp.Write([]byte("foo_int: asdf\nsub_foo_string: 'ok'\nkeepalive_timeout: 3m\nbool: true\n"))
		tmp.Close()

		co.Configure[TestConfigFileStruct](&co.Options{
			Args: []string{"--cool_file", tmp.Name()},
		})
		os.Exit(0)
	}
	assert := assert.New(t)
	stdout, stderr := runExternal(t)

	assert.Equal("", stderr)
	assert.True(strings.HasPrefix(stdout, "error parsing configuration: unsupported config file type: "))
	assert.True(strings.HasSuffix(stdout, "Supported file types are .json, .yml, .yaml\n"))

}
