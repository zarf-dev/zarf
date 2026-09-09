// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestDestroyCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		wantErrContains string
		wantRun         bool
	}{
		{
			name:    "confirmed destroy",
			args:    []string{"--confirm"},
			wantRun: true,
		},
		{
			name:            "positional argument",
			args:            []string{"podinfo", "--confirm"},
			wantErrContains: `unknown command "podinfo"`,
		},
		{
			name:            "missing confirmation",
			args:            []string{},
			wantErrContains: `required flag(s) "confirm" not set`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := newDestroyCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			runCalled := false
			cmd.RunE = func(_ *cobra.Command, _ []string) error {
				runCalled = true
				return nil
			}
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.wantErrContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErrContains)
			}
			require.Equal(t, tt.wantRun, runCalled)
		})
	}
}
