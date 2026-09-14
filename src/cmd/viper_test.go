// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestOptionIsExplicitlySet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		flagConfigured  bool
		viperConfigured bool
		wantConfigured  bool
	}{
		{name: "not configured"},
		{name: "configured by flag", flagConfigured: true, wantConfigured: true},
		{name: "configured by Viper", viperConfigured: true, wantConfigured: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{}
			cmd.Flags().String("registry-url", "", "")
			if tt.flagConfigured {
				require.NoError(t, cmd.Flags().Set("registry-url", "registry.example.com"))
			}

			v := viper.New()
			if tt.viperConfigured {
				v.Set(VInitRegistryURL, "registry.example.com")
			}

			require.Equal(t, tt.wantConfigured, optionIsExplicitlySet(cmd, v, "registry-url", VInitRegistryURL))
		})
	}
}
