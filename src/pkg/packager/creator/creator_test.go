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

func TestLoadPackageDefinition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		testDir     string
		expectedErr string
		creator     Creator
	}{
		{
			name:        "valid package definition",
			testDir:     "valid",
			expectedErr: "",
			creator:     NewPackageCreator(types.ZarfCreateOptions{}, ""),
		},
		{
			name:        "invalid package definition",
			testDir:     "invalid",
			expectedErr: "package must have at least 1 component",
			creator:     NewPackageCreator(types.ZarfCreateOptions{}, ""),
		},
		{
			name:        "valid package definition",
			testDir:     "valid",
			expectedErr: "",
			creator:     NewSkeletonCreator(types.ZarfCreateOptions{}, "", ""),
		},
		{
			name:        "invalid package definition",
			testDir:     "invalid",
			expectedErr: "package must have at least 1 component",
			creator:     NewSkeletonCreator(types.ZarfCreateOptions{}, "", ""),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := layout.New(filepath.Join("testdata", tt.testDir))
			pkg, _, err := tt.creator.LoadPackageDefinition(context.Background(), src)

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
