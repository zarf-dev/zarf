// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestLoadPackageDefinition(t *testing.T) {
	// TODO once creator is refactored to not expect to be in the same directory as the zarf.yaml file
	// this test can be re-parallelized
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
			expectedErr: "package validation failed: package does not contain any compatible components",
			creator:     NewPackageCreator(types.ZarfCreateOptions{}, ""),
		},
		{
			name:        "valid package definition",
			testDir:     "valid",
			expectedErr: "",
			creator:     NewSkeletonCreator(types.ZarfCreateOptions{}, types.ZarfPublishOptions{}),
		},
		{
			name:        "invalid package definition",
			testDir:     "invalid",
			expectedErr: "package validation failed: package does not contain any compatible components",
			creator:     NewSkeletonCreator(types.ZarfCreateOptions{}, types.ZarfPublishOptions{}),
		},
	}
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../../zarf.schema.json")

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cwd, err := os.Getwd()
			require.NoError(t, err)
			defer func() {
				err = os.Chdir(cwd)
				require.NoError(t, err)
			}()
			path := filepath.Join("testdata", tt.testDir)
			err = os.Chdir(path)
			require.NoError(t, err)

			src := layout.New(".")
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
