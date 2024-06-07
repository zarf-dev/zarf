// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestDifferentialPackagePathSetCorrectly(t *testing.T) {
	type testCase struct {
		name     string
		path     string
		cwd      string
		expected string
	}

	absolutePackagePath, err := filepath.Abs(filepath.Join("home", "cool-guy", "zarf-package", "my-package.tar.zst"))
	require.NoError(t, err)

	testCases := []testCase{
		{
			name:     "relative path",
			path:     "my-package.tar.zst",
			cwd:      filepath.Join("home", "cool-guy", "zarf-package"),
			expected: filepath.Join("home", "cool-guy", "zarf-package", "my-package.tar.zst"),
		},
		{
			name:     "absolute path",
			path:     absolutePackagePath,
			cwd:      filepath.Join("home", "should-not-matter"),
			expected: absolutePackagePath,
		},
		{
			name:     "oci path",
			path:     "oci://my-cool-registry.com:555/my-package.tar.zst",
			cwd:      filepath.Join("home", "should-not-matter"),
			expected: "oci://my-cool-registry.com:555/my-package.tar.zst",
		},
		{
			name:     "https path",
			path:     "https://neat-url.com/zarf-init-amd64-v1.0.0.tar.zst",
			cwd:      filepath.Join("home", "should-not-matter"),
			expected: "https://neat-url.com/zarf-init-amd64-v1.0.0.tar.zst",
		},
	}
	for _, testCase := range testCases {
		tc := testCase
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expected, updateRelativeDifferentialPackagePath(tc.path, tc.cwd))
		})
	}
}

func TestLoadPackageDefinition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		testDir     string
		expectedErr string
	}{
		{
			name:        "valid package definition",
			testDir:     "valid",
			expectedErr: "",
		},
		{
			name:        "invalid package definition",
			testDir:     "invalid",
			expectedErr: "package must have at least 1 component",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := layout.New(filepath.Join("testdata", tt.testDir))
			pc := NewPackageCreator(types.ZarfCreateOptions{}, "")
			pkg, _, err := pc.LoadPackageDefinition(context.Background(), src)

			if tt.expectedErr == "" {
				require.NoError(t, err)
				require.NotEmpty(t, pkg)
				return
			}

			require.EqualError(t, err, tt.expectedErr)
			require.Empty(t, pkg)
		})
	}
}
