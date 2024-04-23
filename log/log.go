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

package log

import (
	"context"
	"log/slog"
	"os"
)

var leveler = new(slog.LevelVar)

// Initialize logging
func init() {

	// Create JSON handler
	h := slog.NewJSONHandler(
		os.Stderr,
		&slog.HandlerOptions{
			Level: leveler,
		},
	)

	// Set package variables and default log level
	l := slog.New(h)
	leveler.Set(slog.LevelInfo)

	slog.SetDefault(l)
}

// SetLevel sets the log level
func SetLevel(l slog.Level) {
	leveler.Set(l)
}

// For context value key
type logCtxKeyType struct{}

var logCtxKey = logCtxKeyType{}

// ToContext adds the logger to the context
func ToContext(l *slog.Logger, ctx context.Context) context.Context {
	return context.WithValue(ctx, logCtxKey, l)
}

// FromContext returns the logger from the context. Be sure to check for a nil
// return value in case a logger has not been set with ToContext()
func FromContext(ctx context.Context) *slog.Logger {
	if l := ctx.Value(logCtxKey); l != nil {
		return l.(*slog.Logger)
	}
	return nil
}

// FromContextOrDefault() returns the *slog.Logger from FromContext() or the
// default logger if nil was returned
func FromContextOrDefault(ctx context.Context) *slog.Logger {
	if lg := FromContext(ctx); lg != nil {
		return lg
	}
	return slog.Default()
}
