// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"io"
)

// PausableWriter is a pausable writer
type PausableWriter struct {
	out, wr io.Writer
}

// NewPausableWriter creates a new pausable writer
func NewPausableWriter(wr io.Writer) *PausableWriter {
	return &PausableWriter{out: wr, wr: wr}
}

// Pause sets the output writer to io.Discard
func (pw *PausableWriter) Pause() {
	pw.out = io.Discard
}

// Resume sets the output writer back to the original writer
func (pw *PausableWriter) Resume() {
	pw.out = pw.wr
}

// Write writes the data to the underlying output writer
func (pw *PausableWriter) Write(p []byte) (n int, err error) {
	return pw.out.Write(p)
}
