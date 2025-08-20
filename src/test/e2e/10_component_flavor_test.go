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
	"oras.land/oras-go/v2/registry"
	"sigs.k8s.io/yaml"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestFlavorArchFiltering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		arch        string
		flavor      string
		expectedIDs []string
		Filter      filters.ComponentFilterStrategy
	}{
		{
			name:        "amd64 vanilla",
			arch:        "amd64",
			flavor:      "vanilla",
			expectedIDs: []string{"combined-vanilla-amd", "via-import-vanilla-amd"},
		},
		{
			name:        "amd64 chocolate",
			arch:        "amd64",
			flavor:      "chocolate",
			expectedIDs: []string{"combined-chocolate-amd", "via-import-chocolate-amd"},
		},
		{
			name:        "arm64 chocolate",
			arch:        "arm64",
			flavor:      "chocolate",
			expectedIDs: []string{"combined-chocolate-arm", "via-import-chocolate-arm"},
		},
		{
			name:        "arm64 chocolate with filter",
			arch:        "arm64",
			flavor:      "chocolate",
			expectedIDs: []string{"combined-chocolate-arm"},
			Filter:      filters.BySelectState("combined"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			flavorTest := filepath.Join("src", "test", "packages", "10-package-flavors")
			_, _, err := e2e.Zarf(t, "package", "create", flavorTest, "-o", tmpDir, "--flavor", tt.flavor, "-a", tt.arch, "--no-color", "--confirm")
			require.NoError(t, err)

			tarPath := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-test-package-flavors-%s-v0.0.0-%s.tar.zst", tt.arch, tt.flavor))
			pkgLayout, err := layout.LoadFromTar(context.Background(), tarPath, layout.PackageLayoutOptions{Filter: tt.Filter})
			require.NoError(t, err)
			compIDs := []string{}
			for _, comp := range pkgLayout.Pkg.Components {
				compIDs = append(compIDs, comp.Name+"-"+comp.Description)
			}
			require.ElementsMatch(t, compIDs, tt.expectedIDs)
		})
	}
}

func TestPublishFlavor(t *testing.T) {
	t.Log("E2E: Publish skeleton flavor package")
	t.Parallel()

	var reg registry.Reference
	reg.Registry = testutil.SetupInMemoryRegistry(testutil.TestContext(t), t, 31888)

	ref := reg.String()
	expectedIDs := []string{"combined", "via-import"}

	flavorTest := filepath.Join("src", "test", "packages", "10-package-flavors")
	_, _, err := e2e.Zarf(t, "package", "publish", flavorTest, "--flavor", "vanilla", "--no-color", "oci://"+ref, "--plain-http")
	require.NoError(t, err)

	stdOut, _, err := e2e.Zarf(t, "package", "inspect", "definition", "oci://"+ref+"/test-package-flavors:v0.0.0-vanilla", "--plain-http", "-a", "skeleton")
	require.NoError(t, err)

	var config v1alpha1.ZarfPackage
	err = yaml.Unmarshal([]byte(stdOut), &config)
	require.NoError(t, err)

	for i, component := range config.Components {
		require.Equal(t, expectedIDs[i], component.Name)
	}

	flavorTest = filepath.Join("src", "test", "packages", "10-package-flavors")
	_, _, err = e2e.Zarf(t, "package", "publish", flavorTest, "--flavor", "chocolate", "--no-color", "oci://"+ref, "--plain-http")
	require.NoError(t, err)

	stdOut, _, err = e2e.Zarf(t, "package", "inspect", "definition", "oci://"+ref+"/test-package-flavors:v0.0.0-chocolate", "--plain-http", "-a", "skeleton")
	require.NoError(t, err)

	err = yaml.Unmarshal([]byte(stdOut), &config)
	require.NoError(t, err)

	for i, component := range config.Components {
		require.Equal(t, expectedIDs[i], component.Name)
	}
}
