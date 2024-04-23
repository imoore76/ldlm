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

package lockmap_test

import (
	"reflect"
	"testing"

	"github.com/imoore76/go-ldlm/server/locksrv/lockmap"
	cl "github.com/imoore76/go-ldlm/server/locksrv/lockmap/clientlock"
	"github.com/stretchr/testify/assert"
)

// Check and ensure that all elements exists in another slice and
// check if the length of the slices are equal.
func cmpSlices[T any](aa, bb []T) bool {
	eqCtr := 0
	for _, a := range aa {
		for _, b := range bb {
			if reflect.DeepEqual(a, b) {
				eqCtr++
			}
		}
	}
	if eqCtr != len(bb) || len(aa) != len(bb) {
		return false
	}
	return true
}

// Check and ensure that all clientLocks exists in another slice and
// check if the length of the slices are equal.
func cmpClientLockSlices(aa, bb []cl.ClientLock) bool {
	eqCtr := 0
	for _, a := range aa {
		for _, b := range bb {
			if a.Name() == b.Name() && a.Key() == b.Key() {
				eqCtr++
			}
		}
	}
	if eqCtr != len(bb) || len(aa) != len(bb) {
		return false
	}
	return true

}

type ReadWriter struct {
	Written map[string][]cl.ClientLock
	ToRead  map[string][]cl.ClientLock
}

func (w *ReadWriter) Read() (map[string][]cl.ClientLock, error) {
	return w.ToRead, nil
}

func (w *ReadWriter) Write(locks map[string][]cl.ClientLock) error {
	w.Written = locks
	return nil
}

func TestLockMap(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)
	assert.NotNil(m)
}

func TestAdd(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)

	m.Add("foo", "bar")
	m.Add("you", "there")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
	})
}

func TestRemove(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)

	m.Add("foo", "bar")
	m.Add("you", "there")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
	})

	m.Remove("foo", "bar")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"there": {*cl.New("you", "there")},
		"bar":   {},
	})
}

func TestRemove_NotExist(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)

	m.Add("foo", "bar")
	m.Add("you", "there")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
	})

	m.Remove("asdf", "bar")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
	})
}

func TestRemove_DifferentKey(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)

	m.Add("foo", "bar")
	m.Add("you", "there")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
	})

	defer func() {
		if r := recover(); r != "Client with session key 'asdf' has no clientLocks entry" {
			panic("Unexpected panic: " + r.(string))
		}
	}()

	m.Remove("foo", "asdf")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
	})
}

func TestAddSession(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)

	m.AddSession("asdf")
	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"asdf": {},
	})

}

func TestRemoveSession(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)

	m.Add("foo", "bar")
	m.Add("you", "there")
	m.Add("a", "baz")
	m.Add("b", "baz")
	m.Add("c", "baz")
	m.Add("you", "here")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
		"here":  {*cl.New("you", "here")},
		"baz":   {*cl.New("a", "baz"), *cl.New("b", "baz"), *cl.New("c", "baz")},
	})

	assert.Equal(m.RemoveSession("baz"), []cl.ClientLock{
		*cl.New("a", "baz"),
		*cl.New("b", "baz"),
		*cl.New("c", "baz"),
	})

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
		"here":  {*cl.New("you", "here")},
	})

}

func TestSave(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)

	rwr := new(ReadWriter)
	m.SetReadWriter(rwr)

	m.Add("foo", "bar")
	m.Add("you", "there")
	m.Add("a", "baz")
	m.Add("b", "baz")
	m.Add("c", "baz")
	m.Add("you", "here")

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
		"here":  {*cl.New("you", "here")},
		"baz":   {*cl.New("a", "baz"), *cl.New("b", "baz"), *cl.New("c", "baz")},
	})

	assert.Nil(m.Save())
	assert.Equal(rwr.Written, map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
		"here":  {*cl.New("you", "here")},
		"baz":   {*cl.New("a", "baz"), *cl.New("b", "baz"), *cl.New("c", "baz")},
	})
}

func TestLoad(t *testing.T) {
	assert := assert.New(t)
	conf := new(lockmap.LockMapConfig)
	m := lockmap.New(conf)

	rwr := new(ReadWriter)
	rwr.ToRead = map[string][]cl.ClientLock{
		"bar":   {*cl.New("foo", "bar")},
		"there": {*cl.New("you", "there")},
		"here":  {*cl.New("you", "here")},
		"baz":   {*cl.New("a", "baz"), *cl.New("b", "baz"), *cl.New("c", "baz")},
	}
	m.SetReadWriter(rwr)

	assert.Equal(m.Locks(), map[string][]cl.ClientLock{})

	locks, err := m.Load()
	assert.Nil(err)

	assert.True(cmpClientLockSlices(locks, []cl.ClientLock{
		*cl.New("foo", "bar"),
		*cl.New("you", "there"),
		*cl.New("you", "here"),
		*cl.New("a", "baz"), *cl.New("b", "baz"), *cl.New("c", "baz"),
	}))

	lockMapLocks := m.Locks()

	lmKeys := make([]string, 0, len(lockMapLocks))
	for k, v := range lockMapLocks {
		lmKeys = append(lmKeys, k)
		assert.True(cmpClientLockSlices(v, rwr.ToRead[k]))
	}

	assert.True(cmpSlices[string](lmKeys, []string{"bar", "there", "here", "baz"}))

}
