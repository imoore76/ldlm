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

	cl "github.com/imoore76/go-ldlm/server/clientlock"
	"github.com/imoore76/go-ldlm/server/session"
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

	m.AddLock("foo", "bar", "a")
	m.AddLock("you", "there", "b")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar")},
		"b": {cl.New("you", "there")},
	})
}

func TestRemove(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	m.AddLock("foo", "bar", "a")
	m.AddLock("you", "there", "b")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar")},
		"b": {cl.New("you", "there")},
	})

	m.RemoveLock("foo", "a")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"b": {cl.New("you", "there")},
		"a": {},
	})
}

func TestRemove_NotExist(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	m.AddLock("foo", "bar", "a")
	m.AddLock("you", "there", "b")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar")},
		"b": {cl.New("you", "there")},
	})

	m.RemoveLock("asdf", "a")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar")},
		"b": {cl.New("you", "there")},
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

	m.AddLock("foo", "bar", "a")
	m.AddLock("you", "there", "b")
	m.AddLock("a", "baz", "c")
	m.AddLock("b", "baz", "c")
	m.AddLock("c", "baz", "c")
	m.AddLock("you", "here", "d")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar")},
		"b": {cl.New("you", "there")},
		"d": {cl.New("you", "here")},
		"c": {cl.New("a", "baz"), cl.New("b", "baz"), cl.New("c", "baz")},
	})

	assert.Equal(m.DestroySession("c"), []cl.Lock{
		cl.New("a", "baz"),
		cl.New("b", "baz"),
		cl.New("c", "baz"),
	})

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar")},
		"b": {cl.New("you", "there")},
		"d": {cl.New("you", "here")},
	})

}

func TestSave(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	rwr := new(sessionStore)
	m.SetStore(rwr)

	m.AddLock("foo", "bar", "a")
	m.AddLock("you", "there", "b")
	m.AddLock("a", "baz", "c")
	m.AddLock("b", "baz", "c")
	m.AddLock("c", "baz", "c")
	m.AddLock("you", "here", "d")

	assert.Equal(m.Locks(), map[string][]cl.Lock{
		"a": {cl.New("foo", "bar")},
		"b": {cl.New("you", "there")},
		"d": {cl.New("you", "here")},
		"c": {cl.New("a", "baz"), cl.New("b", "baz"), cl.New("c", "baz")},
	})

	assert.Nil(m.Save())
	assert.Equal(rwr.Written, map[string][]cl.Lock{
		"a": {cl.New("foo", "bar")},
		"b": {cl.New("you", "there")},
		"d": {cl.New("you", "here")},
		"c": {cl.New("a", "baz"), cl.New("b", "baz"), cl.New("c", "baz")},
	})
}

func TestLoad(t *testing.T) {
	assert := assert.New(t)
	conf := new(session.SessionConfig)
	m := session.NewManager(conf)

	rwr := new(sessionStore)
	rwr.ToRead = map[string][]cl.Lock{
		"bar":   {cl.New("foo", "bar")},
		"there": {cl.New("you", "there")},
		"here":  {cl.New("you", "here")},
		"baz":   {cl.New("a", "baz"), cl.New("b", "baz"), cl.New("c", "baz")},
	}
	m.SetStore(rwr)

	assert.Equal(m.Locks(), map[string][]cl.Lock{})

	locks, err := m.Load()
	assert.Nil(err)

	assert.Equal(locks, map[string][]cl.Lock{
		"bar":   {cl.New("foo", "bar")},
		"there": {cl.New("you", "there")},
		"here":  {cl.New("you", "here")},
		"baz":   {cl.New("a", "baz"), cl.New("b", "baz"), cl.New("c", "baz")},
	})

	lockMapLocks := m.Locks()

	assert.Equal(locks, lockMapLocks)

}
