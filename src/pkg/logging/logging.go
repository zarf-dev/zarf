// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package logging contains logging related functionality.
package logging

import (
	"context"
	"io"
	"log/slog"
)

type contextKey struct{}

// NewContext returns a child context containing the given logger.
func NewContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, log)
}

// FromContextOrDiscard returns the logger stored in the context.
func FromContextOrDiscard(ctx context.Context) *slog.Logger {
	v := ctx.Value(contextKey{})
	if v == nil {
		return slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	log, ok := v.(*slog.Logger)
	if !ok {
		// This should never happen.
		panic("unexpected logger type")
	}
	return log
}
