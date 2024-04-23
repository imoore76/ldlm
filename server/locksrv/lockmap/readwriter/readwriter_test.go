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

package readwriter_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
	rw "github.com/imoore76/go-ldlm/server/locksrv/lockmap/readwriter"
)

func newConfig(file string) *rw.Config {
	return &rw.Config{
		StateFile: file,
	}
}

func TestNewReadWriter(t *testing.T) {
	assert := assert.New(t)

	tmpfile, err := os.CreateTemp("", "ldlm-test-*")
	assert.NoError(err)
	defer os.Remove(tmpfile.Name())

	r, err := rw.New(newConfig(tmpfile.Name()))
	assert.NoError(err)
	assert.NotNil(r)
}

func TestReadWriterWriteAndRead(t *testing.T) {
	assert := assert.New(t)

	tmpfile, err := os.CreateTemp("", "ldlm-test-*")
	assert.NoError(err)
	defer os.Remove(tmpfile.Name())

	r, err := rw.New(newConfig(tmpfile.Name()))
	assert.NoError(err)

	c := map[string][]cl.ClientLock{
		"fname": {*cl.New("fname", "fkey")},
		"lname": {*cl.New("fname", "fkey"), *cl.New("lname", "lkey")},
	}

	assert.NoError(r.Write(c))

	c2, err := r.Read()
	assert.NoError(err)
	assert.Equal(c, c2)
}

func TestReadWriterWriteAndRead_EmptyFileName(t *testing.T) {
	assert := assert.New(t)

	r, err := rw.New(&rw.Config{})
	assert.NoError(err)

	c := map[string][]cl.ClientLock{
		"fname": {*cl.New("fname", "fkey")},
		"lname": {*cl.New("fname", "fkey"), *cl.New("lname", "lkey")},
	}

	assert.NoError(r.Write(c))

	c2, err := r.Read()
	assert.NoError(err)
	assert.Equal(map[string][]cl.ClientLock{}, c2)
}
