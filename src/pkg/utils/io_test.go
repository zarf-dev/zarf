// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
)

// GetFinalExecutableCommand returns the final path to the Zarf executable including and library prefixes and overrides.
func TestGetFinalExecutableCommand(t *testing.T) {
	t.Parallel()
	executablePath, err := GetFinalExecutablePath()
	require.NoError(t, err)
	tests := []struct {
		name                    string
		actionCommandZarfPrefix string
		actionUsesSystemZarf    bool
		expected                string
	}{
		{
			name:     "using current binary",
			expected: executablePath,
		},
		{
			name:                    "using prefix takes priority over actionUsesSystemZarf",
			actionCommandZarfPrefix: "my-program",
			expected:                fmt.Sprintf("%s %s", executablePath, "my-program"),
			actionUsesSystemZarf:    true,
		},
		{
			name:                 "using actionUsesSystemZarf",
			actionUsesSystemZarf: true,
			expected:             "zarf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			config.ActionsCommandZarfPrefix = tt.actionCommandZarfPrefix
			config.ActionsUseSystemZarf = tt.actionUsesSystemZarf
			cmd, err := GetFinalExecutableCommand()
			require.NoError(t, err)
			require.Equal(t, tt.expected, cmd)
		})
	}
}
