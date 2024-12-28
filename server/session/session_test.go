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

package session_test

import (
	"testing"

	cl "github.com/imoore76/ldlm/server/clientlock"
	"github.com/imoore76/ldlm/server/session"
	"github.com/stretchr/testify/assert"
)

type sessionStore struct {
	Written map[string][]cl.Lock
	ToRead  map[string][]cl.Lock
}

func (w *sessionStore) Read() (map[string][]cl.Lock, error) {
	return w.ToRead, nil
}

func (w *sessionStore) Write(locks map[string][]cl.Lock) error {
	w.Written = locks
	return nil
}

func TestLockMap(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)
	assert.NotNil(m)
}

func TestAdd(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	m.AddLock("foo", "bar", 1, "a")
	m.AddLock("you", "there", 1, "b")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar", 1)},
		"b": {cl.New("you", "there", 1)},
	})
}

func TestRemove(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	m.AddLock("foo", "bar", 1, "a")
	m.AddLock("you", "there", 1, "b")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar", 1)},
		"b": {cl.New("you", "there", 1)},
	})

	m.RemoveLock("foo", "bar", "a")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"b": {cl.New("you", "there", 1)},
		"a": {},
	})
}

func TestRemove_NotExist(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	m.AddLock("foo", "bar", 1, "a")
	m.AddLock("you", "there", 1, "b")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar", 1)},
		"b": {cl.New("you", "there", 1)},
	})

	m.RemoveLock("asdf", "foo", "a")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar", 1)},
		"b": {cl.New("you", "there", 1)},
	})
}

func TestCreateSession(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	m.CreateSession("asdf")
	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"asdf": {},
	})

}

func TestDestroySession(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	m.AddLock("foo", "bar", 1, "a")
	m.AddLock("you", "there", 1, "b")
	m.AddLock("a", "baz", 1, "c")
	m.AddLock("b", "baz", 1, "c")
	m.AddLock("c", "baz", 1, "c")
	m.AddLock("you", "here", 1, "d")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar", 1)},
		"b": {cl.New("you", "there", 1)},
		"d": {cl.New("you", "here", 1)},
		"c": {cl.New("a", "baz", 1), cl.New("b", "baz", 1), cl.New("c", "baz", 1)},
	})

	assert.Equal(m.DestroySession("c"), []cl.Lock{
		cl.New("a", "baz", 1),
		cl.New("b", "baz", 1),
		cl.New("c", "baz", 1),
	})

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar", 1)},
		"b": {cl.New("you", "there", 1)},
		"d": {cl.New("you", "here", 1)},
	})

}

func TestSave(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	rwr := new(sessionStore)
	m.SetStore(rwr)

	m.AddLock("foo", "bar", 1, "a")
	m.AddLock("you", "there", 1, "b")
	m.AddLock("a", "baz", 1, "c")
	m.AddLock("b", "baz", 1, "c")
	m.AddLock("c", "baz", 1, "c")
	m.AddLock("you", "here", 1, "d")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar", 1)},
		"b": {cl.New("you", "there", 1)},
		"d": {cl.New("you", "here", 1)},
		"c": {cl.New("a", "baz", 1), cl.New("b", "baz", 1), cl.New("c", "baz", 1)},
	})

	assert.Nil(m.Save())
	assert.Equal(rwr.Written, map[string][]cl.Lock{
		"a": {cl.New("foo", "bar", 1)},
		"b": {cl.New("you", "there", 1)},
		"d": {cl.New("you", "here", 1)},
		"c": {cl.New("a", "baz", 1), cl.New("b", "baz", 1), cl.New("c", "baz", 1)},
	})
}

func TestLoad(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	rwr := new(sessionStore)
	rwr.ToRead = map[string][]cl.Lock{
		"bar":   {cl.New("foo", "bar", 1)},
		"there": {cl.New("you", "there", 1)},
		"here":  {cl.New("you", "here", 1)},
		"baz":   {cl.New("a", "baz", 1), cl.New("b", "baz", 1), cl.New("c", "baz", 1)},
	}
	m.SetStore(rwr)

	assert.Equal(m.Locks(), map[string][]cl.Lock{})

	locks, err := m.Load()
	assert.Nil(err)

	assert.Equal(locks, map[string][]cl.Lock{
		"bar":   {cl.New("foo", "bar", 1)},
		"there": {cl.New("you", "there", 1)},
		"here":  {cl.New("you", "here", 1)},
		"baz":   {cl.New("a", "baz", 1), cl.New("b", "baz", 1), cl.New("c", "baz", 1)},
	})

	lockMapLocks := m.Locks()

	assert.Equal(locks, lockMapLocks)

}
