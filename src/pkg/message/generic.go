// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package message provides a rich set of functions for displaying messages to the user.
package message

import "github.com/pterm/pterm"

type Generic struct{}

func (g *Generic) Write(p []byte) (n int, err error) {
	text := string(p)
	pterm.Println(text)
	return len(p), nil
}
