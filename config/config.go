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
This file provides the Config type which can be used to populate a struct with
options specified on the command line or by environment variables. Usage:

	type MyConfig struct {
		StateFile           string        `desc:"File in which to store lock state" default:"" short:"s"`
		DefaultLockTimeout  time.Duration `desc:"Lock timeout to use when loading locks from state file on startup" default:"10m" short:"d"`
		NoClearOnDisconnect bool          `desc:"Do not clear locks on client disconnect" default:"false" short:"c"`
		FooSeconds          uint32        `desc:"Something" default:"10" short:"f"`
		FooInt              int           `desc:"Something" default:"100" short:"o"`
	}

	conf := Configure[MyConfig](&Options{
		EnvPrefix: "MYAPP_",
		Args:      os.Args[1:],
	})

	// `conf` fields are now populated with their defaults or values specified in
	// os.Args or environment variables. Not all golang types have been implemented -
	// only ones needed for ldlm.

cli argument names are converted to snake case. E.g. StateFile becomes
--state_file.

environment variables must be prefixed with the envPrefix specified in
Configure() and are converted to upper case snake. E.g. StateFile with an
envPrefix of "MYAPP_" would be set with `MYAPP_STATE_FILE=/data/locks.state`.

Field tags:
  - desc - The description of the configuration option
  - default - The default value of the configuration option if none is specified
  - short (optional) - The short flag version of the cli option.
*/
package config

import (
	"encoding/json"
	"fmt"
	"os"
	fp "path/filepath"
	"reflect"
	"strings"

	"github.com/fatih/structtag"
	"github.com/iancoleman/strcase"
	flag "github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

type configurer struct {
	config     interface{}
	opts       *Options
	configFile struct {
		Flag  string
		Sflag string
		Value *string
	}
}

// Configuration file type
type File string

// Config options
type Options struct {
	EnvPrefix string
	Args      []string
}

// Configure will populate the supplied struct with options specified on the
// command line or by environment variables prefixed by the specified envPrefix
func Configure[T any](opts *Options) *T {
	c := &configurer{
		config: new(T),
		opts:   opts,
	}

	c.setConfigFile()
	c.setDefaults(c.config)

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("error parsing configuration: %s\n", r)
			os.Exit(1)
		}
	}()

	// Set up a flagset
	f := flag.NewFlagSet("config", flag.ExitOnError)
	f.Usage = func() {
		fmt.Println("Command usage:")
		fmt.Println(f.FlagUsages())
		os.Exit(0)
	}

	// This is a chicken and egg situation where we need to parse flags to
	// determine what the config file flags are, but we want to load the config
	// from the file so that flags overwrite any values set in the config file.
	// loadFlags sets up the flagset and returns setters that actually set the
	// values once args have been parsed
	setters := c.loadFlags(c.config, f)

	// Load config file if the pointer is set
	if c.configFile.Value != nil {
		c.loadConfigFile()
	}

	// Load values from environment
	c.setFromEnv(c.config)

	// Parse into flagset and run setters
	f.Parse(opts.Args)
	for _, fn := range setters {
		fn()
	}
	return c.config.(*T)
}

// setConfigFile checks for a field of type File in the config struct and sets
// the configFile.Value pointer to its address
func (c *configurer) setConfigFile() {
	ft := reflect.TypeOf(new(File))
	c.visitConfigFields(c.config, func(f reflect.StructField, v reflect.Value) (stop bool) {
		if v.Type() == ft {
			c.configFile.Value = (*string)(v.Interface().(*File))
			stop = true
		}
		return stop
	})
}

// setDefaults sets default values specified in field tags
func (c *configurer) setDefaults(s interface{}) {

	c.visitConfigFields(s, func(f reflect.StructField, v reflect.Value) (stop bool) {
		tags, err := structtag.Parse(string(f.Tag))
		if err != nil {
			panic(fmt.Sprintf("setDefaults() error parsing field %s tag: %v", f.Name, err))
		}

		defaultTag, err := tags.Get("default")
		if err != nil {
			panic(fmt.Sprintf("setDefaults(): error parsing field %s 'default' tag :%v", f.Name, err))
		}
		if err := setFromString(v, defaultTag.Value()); err != nil {
			panic(fmt.Sprintf("setDefaults(): %s", err))
		}
		return stop
	})
}

// setFromEnv sets values from environment
func (c *configurer) setFromEnv(s interface{}) {

	c.visitConfigFields(s, func(f reflect.StructField, v reflect.Value) (stop bool) {
		envVal := os.Getenv(
			fmt.Sprintf("%s%s", c.opts.EnvPrefix, strcase.ToScreamingSnake(f.Name)),
		)
		if envVal != "" {
			if err := setFromString(v, envVal); err != nil {
				panic(fmt.Sprintf("setFromEnv(): %s", err))
			}
		}
		return stop
	})
}

// loadFlags() sets field values based on options specified on the command line
// or by environment variables
func (c *configurer) loadFlags(s interface{}, fl *flag.FlagSet) []func() {

	setters := []func(){}

	c.visitConfigFields(s, func(f reflect.StructField, v reflect.Value) (stop bool) {

		tags, err := structtag.Parse(string(f.Tag))
		if err != nil {
			panic(err)
		}

		descTag, err := tags.Get("desc")
		if err != nil {
			panic(fmt.Sprintf("%v: 'desc'", err))
		}
		shortTag, err := tags.Get("short")
		if err != nil {
			// errors in structtag package aren't exported :(
			if err.Error() == "tag does not exist" {
				shortTag = &structtag.Tag{}
			} else {
				panic("Error parsing 'short' tag: " + err.Error())
			}
		}

		fName := strcase.ToSnake(f.Name)
		if v.Elem().Type() == reflect.TypeOf(File("")) {
			c.configFile.Flag = fName
			c.configFile.Sflag = shortTag.Value()
		}

		defaultTag, err := tags.Get("default")
		if err != nil {
			panic(fmt.Sprintf("%v: 'default'", err))
		}

		addToFlagSet(v.Type(), fl, fName, shortTag.Value(), defaultTag.Value(), descTag.Value())

		setters = append(setters, func() {
			if fl.Lookup(fName).Changed {
				if err := setFromString(v, fl.Lookup(fName).Value.String()); err != nil {
					panic(fmt.Sprintf("error parsing flags: %s", err))
				}
			}
		})
		return false
	})
	return setters
}

// Load configuration from config file
func (c *configurer) loadConfigFile() {
	// Set from env since setFromEnv() has not been called yet
	// (chicken and egg)
	if envVal := os.Getenv(
		fmt.Sprintf("%s%s", c.opts.EnvPrefix, strcase.ToScreamingSnake(c.configFile.Flag)),
	); envVal != "" {
		*c.configFile.Value = envVal
	}

	// Set up a flagset that only contains the flags we are looking for to
	// get the config file. Parse args to get the value.
	f := flag.NewFlagSet("configfile", flag.ContinueOnError)
	f.Usage = func() {}
	fileName := f.StringP(
		c.configFile.Flag,
		c.configFile.Sflag,
		*c.configFile.Value,
		"",
	)
	f.Parse(c.opts.Args)

	// No config file specified, nothing to do
	if *fileName == "" {
		return
	}

	// Parse yaml into a generic string interface map
	gmap := make(map[string]interface{})
	confFile, err := os.ReadFile(*fileName)
	if err != nil {
		panic(fmt.Sprintf("error reading config file %s: %v ", *fileName, err))
	}

	// Parse config file based on extension
	confExt := fp.Ext(strings.ToLower(*fileName))
	switch confExt {
	case ".json":
		err = json.Unmarshal(confFile, &gmap)
		if err != nil {
			panic(fmt.Sprintf("error parsing config file: %v", err))
		}
	case ".yml", ".yaml":
		err = yaml.Unmarshal(confFile, gmap)
		if err != nil {
			panic(fmt.Sprintf("error parsing config file: %v", err))
		}
	default:
		panic(fmt.Sprintf("unsupported config file type: %s. Supported file types are .json, .yml, .yaml", fp.Base(*fileName)))
	}

	// Set config struct fields from config values stored in the generic map
	cv := reflect.ValueOf(c.config).Elem()
	for k, v := range gmap {
		fieldName := strcase.ToCamel(k)
		fld := cv.FieldByName(fieldName)
		if fld == (reflect.Value{}) {
			panic(fmt.Sprintf("unknown configuration file field: %s", k))
		}
		if err := setFromString(fld.Addr(), fmt.Sprintf("%v", v)); err != nil {
			panic(fmt.Sprintf("error parsing config file value: %v", err))
		}
	}

}

// visitConfigFields visits the fields of the config struct and calls the
// provided function on each field.
func (c *configurer) visitConfigFields(s interface{}, f func(reflect.StructField, reflect.Value) bool) {
	v := reflect.ValueOf(s).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {

		if !t.Field(i).IsExported() {
			continue
		}

		// Handle anonymous struct fields, which are sub-configs
		if t.Field(i).Anonymous {
			fld := v.Field(i).Addr().Interface()
			c.visitConfigFields(fld, f)
			continue
		}

		if f(t.Field(i), v.Field(i).Addr()) {
			return
		}
	}
}
