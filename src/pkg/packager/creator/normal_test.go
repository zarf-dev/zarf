// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewImportChain(t *testing.T) {
	type testCase struct {
		name     string
		path     string
		cwd      string
		expected string
	}

	testCases := []testCase{
		{
			name:     "relative path",
			path:     "my-package.tar.zst",
			cwd:      "/home/cool-guy/zarf-package",
			expected: "/home/cool-guy/zarf-package/my-package.tar.zst",
		},
		{
			name:     "absolute path",
			path:     "/home/cool-guy/zarf-package/my-package.tar.zst",
			cwd:      "/home/cool-guy/zarf-package",
			expected: "/home/cool-guy/zarf-package/my-package.tar.zst",
		},
		{
			name:     "oci path",
			path:     "oci://my-cool-registry.com:555/my-package.tar.zst",
			cwd:      "/home/cool-guy/zarf-package",
			expected: "oci://my-cool-registry.com:555/my-package.tar.zst",
		},
		{
			name:     "https path",
			path:     "https://neat-url.com/zarf-init-amd64-v1.0.0.tar.zst",
			cwd:      "/home/cool-guy/zarf-package",
			expected: "https://neat-url.com/zarf-init-amd64-v1.0.0.tar.zst",
		},
	}
	for _, testCase := range testCases {
		tc := testCase

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, localizeDifferentialPackagePath(tc.path, tc.cwd))
		})
	}
}
