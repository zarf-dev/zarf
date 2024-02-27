// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import "io"

// Write doesn't do anything but satisfy implementation
func (DiscardProgressWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// UpdateTitle doesn't do anything but satisfy implementation
func (DiscardProgressWriter) UpdateTitle(_ string) {}

// DiscardProgressWriter is a ProgressWriter in which all calls succeed without doing anything
// Use this or nil or if you don't care about writing progress
type DiscardProgressWriter struct{}

// ProgressWriter wraps io.Writer, but also includes an updateTitle function to give the user
// additional context on what's going on. Useful in OCI for tracking layers
type ProgressWriter interface {
	UpdateTitle(string)
	io.Writer
}
