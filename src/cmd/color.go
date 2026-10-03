// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"os"

	"golang.org/x/term"
)

// colorEnvironment captures the terminal properties that decide whether color
// output should be enabled when the user has not chosen explicitly.
type colorEnvironment struct {
	// stdoutIsTerminal is true when stdout is attached to a terminal.
	stdoutIsTerminal bool
	// stderrIsTerminal is true when stderr is attached to a terminal.
	stderrIsTerminal bool
	// term holds the TERM environment variable.
	term string
	// noColor is true when the NO_COLOR environment variable is set to a
	// non-empty value, per https://no-color.org.
	noColor bool
}

// detectColorEnvironment inspects the current process environment.
func detectColorEnvironment() colorEnvironment {
	return colorEnvironment{
		stdoutIsTerminal: term.IsTerminal(int(os.Stdout.Fd())),
		stderrIsTerminal: term.IsTerminal(int(os.Stderr.Fd())),
		term:             os.Getenv("TERM"),
		noColor:          os.Getenv("NO_COLOR") != "",
	}
}

// supportsColor reports whether ANSI color output should be enabled. Zarf
// writes logs to stderr and prints to stdout under a single --no-color
// switch, so color is only enabled when both streams are terminals, the
// NO_COLOR convention is not in use, and the terminal is not "dumb".
func (e colorEnvironment) supportsColor() bool {
	if e.noColor {
		return false
	}
	if e.term == "dumb" {
		return false
	}
	return e.stdoutIsTerminal && e.stderrIsTerminal
}
