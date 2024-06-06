// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"context"
	"log/slog"
)

// ZarfHandler is a simple handler that implements the slog.Handler interface
type ZarfHandler struct{}

// Enabled is always set to true as zarf logging functions are already aware of if they are allowed to be called
func (z ZarfHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// WithAttrs is not supported
func (z ZarfHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return z
}

// WithGroup is not supported
func (z ZarfHandler) WithGroup(_ string) slog.Handler {
	return z
}

// Handle prints the respective logging function in zarf
// This function ignores any key pairs passed through the record
func (z ZarfHandler) Handle(_ context.Context, record slog.Record) error {
	level := record.Level
	message := record.Message

	switch level {
	case slog.LevelDebug:
		Debug(message)
	case slog.LevelInfo:
		Info(message)
	case slog.LevelWarn:
		Warn(message)
	case slog.LevelError:
		Warn(message)
	}
	return nil
}
