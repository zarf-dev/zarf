// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import "github.com/pterm/pterm"

// Generic represents an implementation of the io.Writer interface
type Generic struct{}

// Write writes a byte array to stdout and returns the number of bytes written
func (g *Generic) Write(p []byte) (n int, err error) {
	text := string(p)
	pterm.Println(text)
	return len(p), nil
}
