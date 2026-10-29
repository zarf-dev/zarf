// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helpers

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveArch(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		archs []string
		want  string
	}{
		{"first nonempty", []string{"", "arm64", "amd64"}, "arm64"},
		{"unchanged", []string{"custom-arch"}, "custom-arch"},
		{"empty", []string{"", ""}, runtime.GOARCH},
		{"omitted", nil, runtime.GOARCH},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ResolveArch(tt.archs...))
		})
	}
}
