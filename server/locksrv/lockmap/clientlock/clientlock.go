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
This file contains the ClientLock struct definition and methods. A ClientLock
represents a lock held by an RPC client.
*/
package clientlock

// Client lock represents a lock held by an RPC client
type ClientLock struct {
	name string
	key  string
}

func (c *ClientLock) Name() string {
	return c.name
}

func (c *ClientLock) Key() string {
	return c.key
}

func New(name string, key string) *ClientLock {
	return &ClientLock{name: name, key: key}
}
