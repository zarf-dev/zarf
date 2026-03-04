// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUseInsecureRegistryTransport(t *testing.T) {
	tests := []struct {
		name          string
		commandFlag   bool
		globalFlag    bool
		expectedValue bool
	}{
		{
			name:          "both flags false",
			commandFlag:   false,
			globalFlag:    false,
			expectedValue: false,
		},
		{
			name:          "command flag true",
			commandFlag:   true,
			globalFlag:    false,
			expectedValue: true,
		},
		{
			name:          "global flag true",
			commandFlag:   false,
			globalFlag:    true,
			expectedValue: true,
		},
		{
			name:          "both flags true",
			commandFlag:   true,
			globalFlag:    true,
			expectedValue: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := useInsecureRegistryTransport(tc.commandFlag, tc.globalFlag)
			require.Equal(t, tc.expectedValue, actual)
		})
	}
}
