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

func TestGetArches(t *testing.T) {
	t.Parallel()
	t.Run("empty falls back to runtime.GOARCH", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, []string{runtime.GOARCH}, GetArches(""))
	})
	t.Run("non-empty parses normally", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, []string{"amd64", "arm64"}, GetArches("amd64,arm64"))
	})
}
