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
		name      string
		testDir   string
		expectErr bool
	}{
		{
			name:      "valid package definition",
			testDir:   "valid",
			expectErr: false,
		},
		{
			name:      "invalid package definition",
			testDir:   "invalid",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := layout.New(filepath.Join("testdata", tt.testDir))
			pc := NewSkeletonCreator(types.ZarfCreateOptions{}, types.ZarfPublishOptions{})
			pkg, _, err := pc.LoadPackageDefinition(src)

			switch {
			case tt.expectErr:
				require.Error(t, err)
			default:
				require.NoError(t, err)
				require.NotEmpty(t, pkg)
			}
		})
	}
}
