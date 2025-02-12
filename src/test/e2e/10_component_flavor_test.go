// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
)

func TestFlavorArchFiltering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arch        string
		flavor      string
		expectedIDs []string
	}{
		{
			arch:        "amd64",
			flavor:      "vanilla",
			expectedIDs: []string{"combined-vanilla-amd", "via-import-vanilla-amd"},
		},
		{
			arch:        "amd64",
			flavor:      "chocolate",
			expectedIDs: []string{"combined-chocolate-amd", "via-import-chocolate-amd"},
		},
		{
			arch:        "arm64",
			flavor:      "chocolate",
			expectedIDs: []string{"combined-chocolate-arm", "via-import-chocolate-arm"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.arch+"-"+tt.flavor, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			flavorTest := filepath.Join("src", "test", "packages", "10-package-flavors")
			_, _, err := e2e.Zarf(t, "package", "create", flavorTest, "-o", tmpDir, "--flavor", tt.flavor, "-a", tt.arch, "--no-color", "--confirm")
			require.NoError(t, err)

			tarPath := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-test-package-flavors-%s.tar.zst", tt.arch))
			pkgLayout, err := layout2.LoadFromTar(context.Background(), tarPath, layout2.PackageLayoutOptions{})
			require.NoError(t, err)
			compIDs := []string{}
			for _, comp := range pkgLayout.Pkg.Components {
				compIDs = append(compIDs, comp.Name+"-"+comp.Description)
			}
			require.ElementsMatch(t, compIDs, tt.expectedIDs)
		})
	}
}
