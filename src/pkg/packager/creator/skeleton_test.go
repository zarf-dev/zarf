// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestSkeletonLoadPackageDefinition(t *testing.T) {
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
			sc := NewSkeletonCreator(types.ZarfCreateOptions{}, types.ZarfPublishOptions{})
			pkg, _, err := sc.LoadPackageDefinition(src)

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
