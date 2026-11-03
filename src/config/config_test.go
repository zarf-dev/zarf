// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package config

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitPackageNameArchitecture(t *testing.T) {
	for _, architecture := range []string{"arm64", "amd64", ""} {
		want := architecture
		if want == "" {
			want = runtime.GOARCH
		}
		require.Equal(t, fmt.Sprintf("zarf-init-%s-%s.tar.zst", want, CLIVersion), GetInitPackageName(architecture))
	}
}
