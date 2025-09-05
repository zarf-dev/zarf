// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package template provides functions for templating yaml files.
package template

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/variables"
)

func TestGetSanitizedTemplateMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]*variables.TextTemplate
		expected map[string]string
	}{
		{
			name: "Sensitive entry",
			input: map[string]*variables.TextTemplate{
				"###SENSITIVE###": {Sensitive: true, Value: "secret"},
			},
			expected: map[string]string{
				"###SENSITIVE###": "**sanitized**",
			},
		},
		{
			name: "Non-sensitive entries",
			input: map[string]*variables.TextTemplate{
				"###VARIABLE###": {Sensitive: false, Value: "value"},
			},
			expected: map[string]string{
				"###VARIABLE###": "value",
			},
		},
		{
			name: "Sensitive and non-sensitive entries",
			input: map[string]*variables.TextTemplate{
				"###ZARF_GIT_AUTH_PULL###": {Sensitive: true, Value: "secret1"},
				"###ZARF_GIT_AUTH_PUSH###": {Sensitive: true, Value: "secret2"},
				"###ZARF_GIT_PUSH###":      {Sensitive: false, Value: "zarf-git-user"},
				"###ZARF_GIT_PULL###":      {Sensitive: false, Value: "zarf-git-read-user"},
			},
			expected: map[string]string{
				"###ZARF_GIT_AUTH_PULL###": "**sanitized**",
				"###ZARF_GIT_AUTH_PUSH###": "**sanitized**",
				"###ZARF_GIT_PULL###":      "zarf-git-read-user",
				"###ZARF_GIT_PUSH###":      "zarf-git-user",
			},
		},
		{
			name:     "Nil map",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:     "Empty map",
			input:    map[string]*variables.TextTemplate{},
			expected: map[string]string{},
		},
		{
			name: "Map with nil value",
			input: map[string]*variables.TextTemplate{
				"###ZARF_GIT_AUTH_PULL###": nil,
			},
			expected: map[string]string{
				"###ZARF_GIT_AUTH_PULL###": "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := getSanitizedTemplateMap(test.input)
			require.Equal(t, test.expected, output)
		})
	}
}

func TestGetZarfTemplatesForIPv6SeedRegistry(t *testing.T) {
	tests := []struct {
		name                    string
		ipv6Enabled             bool
		expectedRegistryAddress string
		expectedIPv6Enabled     string
	}{
		{
			name:                    "IPv4",
			ipv6Enabled:             false,
			expectedRegistryAddress: "127.0.0.1:31997",
			expectedIPv6Enabled:     "false",
		},
		{
			name:                    "IPv6",
			ipv6Enabled:             true,
			expectedRegistryAddress: "[::1]:31997",
			expectedIPv6Enabled:     "true",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config.ZarfSeedPort = 31997
			state := state.State{
				RegistryInfo: state.RegistryInfo{
					Address:  test.expectedRegistryAddress,
					NodePort: 31997,
				},
				IPv6Enabled: test.ipv6Enabled,
			}
			templateMap, err := GetZarfTemplates(context.Background(), "zarf-seed-registry", &state)
			require.NoError(t, err)
			require.Equal(t, test.expectedRegistryAddress, templateMap["###ZARF_SEED_REGISTRY###"].Value)
			require.Equal(t, test.expectedIPv6Enabled, templateMap["###ZARF_IPV6_ENABLED###"].Value)
			require.NotEmpty(t, templateMap["###ZARF_HTPASSWD###"].Value)
		})
	}
}
