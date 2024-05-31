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

package clientlock_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cl "github.com/imoore76/go-ldlm/server/clientlock"
)

func TestClientLock(t *testing.T) {
	assert := assert.New(t)
	var lk cl.Lock = cl.New("foo", "bar", 22)

	assert.Equal("foo", lk.Name())
	assert.Equal("bar", lk.Key())
	assert.Equal(int32(22), lk.Size())
}
