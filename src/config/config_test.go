// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package config

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseArchitectures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: nil},
		{name: "single", in: "amd64", want: []string{"amd64"}},
		{name: "trims whitespace", in: " amd64 , arm64 ", want: []string{"amd64", "arm64"}},
		{name: "drops empty entries", in: "amd64,,arm64", want: []string{"amd64", "arm64"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, ParseArchitectures(tt.in))
		})
	}
}

func TestParseArchitecturesOrDefault(t *testing.T) {
	t.Run("non-empty parses normally", func(t *testing.T) {
		require.Equal(t, []string{"amd64", "arm64"}, ParseArchitecturesOrDefault("amd64,arm64"))
	})
	t.Run("empty falls back to runtime.GOARCH when CLIArch is unset", func(t *testing.T) {
		original := CLIArch
		CLIArch = ""
		t.Cleanup(func() { CLIArch = original })
		require.Equal(t, []string{runtime.GOARCH}, ParseArchitecturesOrDefault(""))
	})
	t.Run("empty falls back to CLIArch when set", func(t *testing.T) {
		original := CLIArch
		CLIArch = "ppc64le"
		t.Cleanup(func() { CLIArch = original })
		require.Equal(t, []string{"ppc64le"}, ParseArchitecturesOrDefault(""))
	})
}
