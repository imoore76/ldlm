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
This file contains a Lock struct definition which represents a lock held by a lock server client.
*/
package clientlock

// Lock represents a lock held by a lock server client
type Lock struct {
	name string
	key  string
	size int32
}

// Name returns the name of the lock
func (c Lock) Name() string {
	return c.name
}

// Key returns the key for the lock
func (c Lock) Key() string {
	return c.key
}

// Size returns the size of the lock
func (c Lock) Size() int32 {
	return c.size
}

// New creates a new Lock with the provided name and key
func New(name string, key string, size int32) Lock {
	return Lock{name: name, key: key, size: size}
}
