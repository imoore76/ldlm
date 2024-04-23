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
This file contains handlers for reflect types handled in configuration parsing
*/
package config

import (
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"

	flag "github.com/spf13/pflag"
)

var logLevelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// flagSetMap holds a map of reflect types to functions that add a flag to a flagset
var flagSetMap = map[reflect.Type]func(fs *flag.FlagSet, name string, short string, def string, desc string){
	// Duration
	reflect.TypeOf(new(time.Duration)): func(fs *flag.FlagSet, name string, short string, def string, desc string) {
		d := new(time.Duration)
		setFromString(reflect.ValueOf(d), def)
		fs.DurationP(name, short, *d, desc)
	},
	// String
	reflect.TypeOf(new(string)): func(fs *flag.FlagSet, name string, short string, def string, desc string) {
		fs.StringP(name, short, def, desc)
	},
	// File (config file)
	reflect.TypeOf(new(File)): func(fs *flag.FlagSet, name string, short string, def string, desc string) {
		fs.StringP(name, short, def, desc)
	},
	// uint32
	reflect.TypeOf(new(uint32)): func(fs *flag.FlagSet, name string, short string, def string, desc string) {
		d := new(uint32)
		setFromString(reflect.ValueOf(d), def)
		fs.Uint32P(name, short, *d, desc)
	},
	// bool
	reflect.TypeOf(new(bool)): func(fs *flag.FlagSet, name string, short string, def string, desc string) {
		d := new(bool)
		setFromString(reflect.ValueOf(d), def)
		fs.BoolP(name, short, *d, desc)
	},
	// slog.Level
	reflect.TypeOf(new(slog.Level)): func(fs *flag.FlagSet, name string, short string, def string, desc string) {
		fs.StringP(name, short, def, desc)
	},
}

// setFromStringMap holds a map of reflect types to functions that set a value from a string
var setFromStringMap = map[reflect.Type]func(reflect.Value, string) error{
	// Duration
	reflect.TypeOf(new(time.Duration)): func(v reflect.Value, s string) error {
		def, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid duration: %s", s)
		}
		v.Elem().SetInt(int64(def))
		return nil
	},
	// String
	reflect.TypeOf(new(string)): func(v reflect.Value, s string) error {
		v.Elem().SetString(s)
		return nil
	},
	// File (config file)
	reflect.TypeOf(new(File)): func(v reflect.Value, s string) error {
		v.Elem().SetString(s)
		return nil
	},
	// uint32
	reflect.TypeOf(new(uint32)): func(v reflect.Value, s string) error {
		def, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return fmt.Errorf("unable to parse uint32 value: %s: %s", s, err)
		}
		v.Elem().SetUint(def)
		return nil
	},
	// bool
	reflect.TypeOf(new(bool)): func(v reflect.Value, s string) error {
		if b, err := strconv.ParseBool(s); err != nil {
			return fmt.Errorf("unable to parse bool value: %s: %s", s, err)
		} else {
			v.Elem().SetBool(b)
		}
		return nil
	},
	// slog.Level
	reflect.TypeOf(new(slog.Level)): func(v reflect.Value, s string) error {
		lvl, ok := logLevelMap[strings.ToLower(s)]
		if !ok {
			return fmt.Errorf("invalid value for log level '%s'", s)
		}
		v.Elem().SetInt(int64(lvl))
		return nil
	},
}

// setFromString sets the value of a reflect.Value (which is a pointer to a
// value) from a string
//
// v: reflect.Value,
// s: string
// returns error
func setFromString(v reflect.Value, s string) error {
	if fn, ok := setFromStringMap[v.Type()]; !ok {
		return fmt.Errorf("unsupported setFromStringMap type: %v", v.Type())
	} else {
		return fn(v, s)
	}
}

// addToFlagSet adds a flag to the provided FlagSet based on the given type.
//
// t: the reflect.Type of the flag
// fs: the pointer to the flag.FlagSet to add the flag to
// name: the name of the flag
// short: the short name of the flag
// def: the default value of the flag
// desc: the description of the flag
func addToFlagSet(t reflect.Type, fs *flag.FlagSet, name string, short string, def string, desc string) {
	if fn, ok := flagSetMap[t]; !ok {
		panic(fmt.Sprintf("unsupported flagSetMap type: %v", t))
	} else {
		fn(fs, name, short, def, desc)
	}
}
