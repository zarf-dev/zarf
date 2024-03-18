// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import (
	"io"
	"os"
)

// pausableLogFile is a pausable log file
type pausableLogFile struct {
	wr io.Writer
	f  *os.File
}

// pause the log file
func (l *pausableLogFile) pause() {
	l.wr = io.Discard
}

// resume the log file
func (l *pausableLogFile) resume() {
	l.wr = l.f
}

// Write writes the data to the log file
func (l *pausableLogFile) Write(p []byte) (n int, err error) {
	return l.wr.Write(p)
}
