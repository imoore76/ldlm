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

package timer_test

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thejerf/slogassert"

	"github.com/imoore76/go-ldlm/lock/timer"
)

func TestManager(t *testing.T) {
	assert := assert.New(t)
	expired := []string{}
	m, cl := timer.NewManager(func(n string, k string) (bool, error) {
		expired = append(expired, n+":"+k)
		return true, nil
	})
	defer cl()

	m.Add("foo", "bar", 1*time.Millisecond)
	m.Add("me", "you", 10*time.Millisecond)
	m.Add("baz", "foo", 1*time.Hour)
	time.Sleep(100 * time.Millisecond)

	assert.Equal([]string{"foo:bar", "me:you"}, expired)
}

func TestManager_Refresh(t *testing.T) {
	assert := assert.New(t)
	expired := []string{}
	m, cl := timer.NewManager(func(n string, k string) (bool, error) {
		expired = append(expired, n+":"+k)
		return true, nil
	})
	defer cl()

	m.Add("foo", "bar", 1*time.Minute)
	m.Add("me", "you", 1*time.Second)
	m.Add("baz", "foo", 1*time.Hour)

	ok, err := m.Refresh("me", "you", 1*time.Hour)
	assert.Nil(err)
	assert.True(ok)

	time.Sleep(1500 * time.Millisecond)

	// Nothing has expired
	assert.Equal([]string{}, expired)
}

func TestManager_RefreshWrongKey(t *testing.T) {
	assert := assert.New(t)
	expired := []string{}
	m, cl := timer.NewManager(func(n string, k string) (bool, error) {
		expired = append(expired, n+":"+k)
		return true, nil
	})
	defer cl()

	m.Add("foo", "bar", 1*time.Minute)
	m.Add("me", "you", 1*time.Hour)
	m.Add("baz", "foo", 1*time.Hour)

	ok, err := m.Refresh("foo", "nope", 1*time.Hour)
	assert.False(ok)
	assert.Equal(err, timer.ErrLockDoesNotExistOrInvalidKey)

}
func TestManager_Remove(t *testing.T) {
	assert := assert.New(t)
	expired := []string{}
	m, cl := timer.NewManager(func(n string, k string) (bool, error) {
		expired = append(expired, n+":"+k)
		return true, nil
	})
	defer cl()

	m.Add("foo", "bar", 1*time.Hour)
	m.Add("me", "you", 1*time.Second)
	m.Add("baz", "foo", 1*time.Hour)

	m.Remove("me", "you")
	time.Sleep(1500 * time.Millisecond)

	// me:you should have been removed before it expired
	assert.Equal([]string{}, expired)

	m.Remove("not", "there") // should do nothing
}

func TestManager_Shutdown(t *testing.T) {
	assert := assert.New(t)
	expired := []string{}
	m, cl := timer.NewManager(func(n string, k string) (bool, error) {
		expired = append(expired, n+":"+k)
		return true, nil
	})

	m.Add("foo", "bar", 1*time.Second)
	m.Add("me", "you", 1*time.Second)
	m.Add("baz", "foo", 1*time.Second)

	cl()
	time.Sleep(1500 * time.Millisecond)

	// Nothing has expired because timers were stopped
	assert.Equal([]string{}, expired)
}

func TestManager_TimeoutFuncFalse(t *testing.T) {
	assert := assert.New(t)
	expired := []string{}

	logger := slogassert.New(t, slog.LevelError, nil)
	pre := slog.Default()
	slog.SetDefault(slog.New(logger))
	defer slog.SetDefault(pre)

	m, cl := timer.NewManager(func(n string, k string) (bool, error) {
		expired = append(expired, n+":"+k)
		return false, nil
	})
	defer cl()

	m.Add("foo", "bar", 1*time.Millisecond)
	m.Add("me", "you", 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	// Failures were logged
	logger.AssertPrecise(slogassert.LogMessageMatch{
		Message:       "Error running onTimeoutFunc() on lock timeout",
		Level:         slog.LevelError,
		Attrs:         map[string]any{"lock": "foo", "key": "bar"},
		AllAttrsMatch: false,
	})
	logger.AssertPrecise(slogassert.LogMessageMatch{
		Message:       "Error running onTimeoutFunc() on lock timeout",
		Level:         slog.LevelError,
		Attrs:         map[string]any{"lock": "me", "key": "you"},
		AllAttrsMatch: false,
	})

	// These were passed to the function, but nothing panic()d when it returned
	// false
	assert.Equal([]string{"foo:bar", "me:you"}, expired)
}

func TestManager_TimeoutFuncError(t *testing.T) {
	assert := assert.New(t)
	expired := []string{}

	logger := slogassert.New(t, slog.LevelError, nil)
	pre := slog.Default()
	slog.SetDefault(slog.New(logger))
	defer slog.SetDefault(pre)

	oopsErr := errors.New("oops")
	m, cl := timer.NewManager(func(n string, k string) (bool, error) {
		expired = append(expired, n+":"+k)
		return false, oopsErr
	})
	defer cl()

	m.Add("foo", "bar", 1*time.Millisecond)
	m.Add("me", "you", 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	// Failures were logged
	logger.AssertPrecise(slogassert.LogMessageMatch{
		Message:       "Error running onTimeoutFunc() on lock timeout",
		Level:         slog.LevelError,
		Attrs:         map[string]any{"lock": "foo", "key": "bar"},
		AllAttrsMatch: false,
	})
	logger.AssertPrecise(slogassert.LogMessageMatch{
		Message:       "Error running onTimeoutFunc() on lock timeout",
		Level:         slog.LevelError,
		Attrs:         map[string]any{"lock": "me", "key": "you"},
		AllAttrsMatch: false,
	})

	// These were passed to the function, but nothing panic()d when it returned
	// false
	assert.Equal([]string{"foo:bar", "me:you"}, expired)
}
