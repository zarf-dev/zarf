// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetBaseDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		want            string
		wantErrContains string
	}{
		{
			name: "no args defaults to current directory",
			args: nil,
			want: ".",
		},
		{
			name: "directory arg",
			args: []string{"path/to/definition"},
			want: "path/to/definition",
		},
		{
			name: "explicit zarf.yaml",
			args: []string{"path/to/zarf.yaml"},
			want: "path/to/zarf.yaml",
		},
		{
			name:            "rejects built tar.zst package",
			args:            []string{"zarf-package-foo-amd64.tar.zst"},
			wantErrContains: "is a built Zarf package",
		},
		{
			name:            "rejects built tar package",
			args:            []string{"zarf-package-foo-amd64.tar"},
			wantErrContains: "is a built Zarf package",
		},
		{
			name:            "rejects split package",
			args:            []string{"zarf-package-foo-amd64.part000"},
			wantErrContains: "is a split Zarf package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := setBaseDirectory(tt.args)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErrContains)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
