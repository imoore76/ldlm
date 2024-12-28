// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/imoore76/ldlm/log"
	"github.com/stretchr/testify/assert"
)

func Test_Context(t *testing.T) {

	nilog := log.FromContext(context.Background())
	assert.Nil(t, nilog)

	logA := log.FromContextOrDefault(context.Background())

	logB := slog.Default().With("testattr", "testval")
	ctx := log.ToContext(logB, context.Background())

	logC := log.FromContext(ctx)
	assert.Equal(t, logC, logB)
	assert.NotEqual(t, logC, logA)
}
