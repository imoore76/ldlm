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

package store_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	cl "github.com/imoore76/ldlm/server/clientlock"
	"github.com/imoore76/ldlm/server/session/store"
)

func TestNewStore(t *testing.T) {
	assert := assert.New(t)

	tmpfile, err := os.CreateTemp("", "ldlm-test-*")
	assert.NoError(err)
	defer os.Remove(tmpfile.Name())

	r, err := store.New(tmpfile.Name())
	assert.NoError(err)
	assert.NotNil(r)
}

func TestStoreWriteAndRead(t *testing.T) {
	assert := assert.New(t)

	tmpfile, err := os.CreateTemp("", "ldlm-test-*")
	assert.NoError(err)
	defer os.Remove(tmpfile.Name())

	r, err := store.New(tmpfile.Name())
	assert.NoError(err)

	c := map[string][]cl.Lock{
		"fname": {cl.New("fname", "fkey", 1)},
		"lname": {cl.New("fname", "fkey", 1), cl.New("lname", "lkey", 1)},
	}

	assert.NoError(r.Write(c))

	c2, err := r.Read()
	assert.NoError(err)
	assert.Equal(c, c2)
}

func TestStoreWriteAndRead_EmptyFileName(t *testing.T) {
	assert := assert.New(t)

	r, err := store.New("")
	assert.NoError(err)

	c := map[string][]cl.Lock{
		"fname": {cl.New("fname", "fkey", 1)},
		"lname": {cl.New("fname", "fkey", 1), cl.New("lname", "lkey", 1)},
	}

	assert.NoError(r.Write(c))

	c2, err := r.Read()
	assert.NoError(err)
	assert.Equal(map[string][]cl.Lock(nil), c2)
}
