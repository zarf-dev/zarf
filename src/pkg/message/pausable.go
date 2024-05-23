// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"io"
	"os"
)

// PausableLogFile is a pausable log file
type PausableLogFile struct {
	wr io.Writer
	f  *os.File
}

// NewPausableLogFile creates a new pausable log file
func NewPausableLogFile(f *os.File) *PausableLogFile {
	return &PausableLogFile{wr: f, f: f}
}

// Pause the log file
func (l *PausableLogFile) Pause() {
	l.wr = io.Discard
}

// Resume the log file
func (l *PausableLogFile) Resume() {
	l.wr = l.f
}

// Write writes the data to the log file
func (l *PausableLogFile) Write(p []byte) (n int, err error) {
	return l.wr.Write(p)
}
