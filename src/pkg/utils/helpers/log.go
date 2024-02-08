// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import "io"

// Debug does nothing
func (l *DiscardLogger) Debug(_ string, _ ...any) {}

// Info does nothing
func (l *DiscardLogger) Info(_ string, _ ...any) {}

// Warn does nothing
func (l *DiscardLogger) Warn(_ string, _ ...any) {}

// Error does nothing
func (l *DiscardLogger) Error(_ string, _ ...any) {}

// DiscardLogger is the default if the WithLogger modifier is not used
// It discards all logs
type DiscardLogger struct{}

// Write doesn't do anything but satisfy implementation
func (DiscardProgressWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// UpdateTitle doesn't do anything but satisfy implementation
func (DiscardProgressWriter) UpdateTitle(_ string) {}

// DiscardProgressWriter is a ProgressWriter in which all calls succeed without doing anything
// Use this or nil or if you don't care about writing progress
type DiscardProgressWriter struct{}

// Logger allows for varying levels of logging depending on importance
// Arguments are intended to be used in printf-style
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// ProgressWriter wraps io.Writer, but also includes an updateTitle function to give the user
// additional context on what's going on. Useful in OCI for tracking layers
type ProgressWriter interface {
	UpdateTitle(string)
	io.Writer
}
