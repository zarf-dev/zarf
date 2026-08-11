// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestDevDeploy_filtersComponents(t *testing.T) {
	ctx := testutil.TestContext(t)
	packageDir := t.TempDir()
	source := filepath.Join(packageDir, "source.txt")
	require.NoError(t, os.WriteFile(source, []byte("test data"), 0o600))

	selectedTarget := filepath.Join(t.TempDir(), "selected.txt")
	unselectedTarget := filepath.Join(t.TempDir(), "unselected.txt")
	required := false
	pkg := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.ZarfPackageConfig,
		Metadata:   v1alpha1.ZarfMetadata{Name: "dev-deploy-filter"},
		Components: []v1alpha1.ZarfComponent{
			{
				Name:     "selected",
				Required: &required,
				Files:    []v1alpha1.ZarfFile{{Source: source, Target: selectedTarget}},
			},
			{
				Name:     "unselected",
				Required: helpers.BoolPtr(false),
				Files:    []v1alpha1.ZarfFile{{Source: source, Target: unselectedTarget}},
			},
		},
	}
	b, err := goyaml.Marshal(pkg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), b, 0o600))

	err = DevDeploy(ctx, packageDir, DevDeployOptions{
		OptionalComponents: "selected",
		CachePath:          filepath.Join(packageDir, "cache"),
	})

	require.NoError(t, err)
	require.FileExists(t, selectedTarget)
	require.NoFileExists(t, unselectedTarget)
}
