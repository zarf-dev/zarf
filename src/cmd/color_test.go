// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupportsColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  colorEnvironment
		want bool
	}{
		{
			name: "interactive terminal supports color",
			env:  colorEnvironment{stdoutIsTerminal: true, stderrIsTerminal: true, term: "xterm-256color"},
			want: true,
		},
		{
			name: "unset TERM still supports color when interactive",
			env:  colorEnvironment{stdoutIsTerminal: true, stderrIsTerminal: true},
			want: true,
		},
		{
			name: "NO_COLOR disables color",
			env:  colorEnvironment{stdoutIsTerminal: true, stderrIsTerminal: true, term: "xterm", noColor: true},
			want: false,
		},
		{
			name: "dumb terminal disables color",
			env:  colorEnvironment{stdoutIsTerminal: true, stderrIsTerminal: true, term: "dumb"},
			want: false,
		},
		{
			name: "piped stdout disables color",
			env:  colorEnvironment{stdoutIsTerminal: false, stderrIsTerminal: true, term: "xterm"},
			want: false,
		},
		{
			name: "redirected stderr disables color",
			env:  colorEnvironment{stdoutIsTerminal: true, stderrIsTerminal: false, term: "xterm"},
			want: false,
		},
		{
			name: "no terminal at all disables color",
			env:  colorEnvironment{},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.env.supportsColor())
		})
	}
}

func TestDetectColorEnvironment(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	env := detectColorEnvironment()
	require.True(t, env.noColor)
	require.Equal(t, "dumb", env.term)
	// Test processes do not run with a terminal attached to stdout/stderr.
	require.False(t, env.stdoutIsTerminal)
	require.False(t, env.stderrIsTerminal)
	require.False(t, env.supportsColor())

	// NO_COLOR must be non-empty to count, per https://no-color.org.
	t.Setenv("NO_COLOR", "")
	env = detectColorEnvironment()
	require.False(t, env.noColor)
}
