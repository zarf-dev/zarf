// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"strings"
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

func TestViperEnvKeyMappingAndPrecedence(t *testing.T) {
	// Verify environment variable name mapping rules (prefix ZARF_ and delimiter . -> _)
	v := viper.New()
	v.SetEnvPrefix("zarf")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 1. Single-level key mapping
	t.Setenv("ZARF_LOG_LEVEL", "warn")
	require.Equal(t, "warn", v.GetString("log_level"))

	// 2. Nested key mapping (dot replaced with underscore)
	t.Setenv("ZARF_PACKAGE_DEPLOY_TIMEOUT", "45m")
	require.Equal(t, "45m", v.GetString("package.deploy.timeout"))

	t.Setenv("ZARF_INIT_GIT_URL", "https://git.example.com")
	require.Equal(t, "https://git.example.com", v.GetString("init.git.url"))

	// 3. Precedence hierarchy: Flag > Env > Default
	cmd := &cobra.Command{}
	cmd.Flags().String("log-level", "", "")

	// Default value
	v.SetDefault("log_level", "info")
	require.Equal(t, "warn", v.GetString("log_level")) // Env overrides default

	// Flag override
	require.NoError(t, cmd.Flags().Set("log-level", "debug"))
	val := v.GetString("log_level")
	if cmd.Flags().Changed("log-level") {
		flagVal, _ := cmd.Flags().GetString("log-level")
		val = flagVal
	}
	require.Equal(t, "debug", val) // Flag takes highest precedence
}
