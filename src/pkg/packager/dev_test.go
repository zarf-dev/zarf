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
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/value"
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

func TestDevDeploy_appliesValues(t *testing.T) {
	ctx := testutil.TestContext(t)

	packageDir := t.TempDir()
	source := filepath.Join(packageDir, "source.txt")
	require.NoError(t, os.WriteFile(source, []byte("{{ .Values.message }}"), 0o600))

	target := filepath.Join(t.TempDir(), "rendered.txt")
	pkg := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.ZarfPackageConfig,
		Metadata:   v1alpha1.ZarfMetadata{Name: "dev-deploy-values"},
		Components: []v1alpha1.ZarfComponent{{
			Name:     "values",
			Required: helpers.BoolPtr(true),
			Files: []v1alpha1.ZarfFile{{
				Source:   source,
				Target:   target,
				Template: helpers.BoolPtr(true),
			}},
		}},
	}
	b, err := goyaml.Marshal(pkg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), b, 0o600))

	err = DevDeploy(ctx, packageDir, DevDeployOptions{
		CachePath: filepath.Join(packageDir, "cache"),
		Values:    value.Values{"message": "from-deploy-values"},
	})

	require.NoError(t, err)
	contents, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "from-deploy-values\n", string(contents))
}

func TestDevDeploy_runsV1Beta1Actions(t *testing.T) {
	ctx := testutil.TestContext(t)
	packageDir := t.TempDir()
	pkg := v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata:   v1beta1.PackageMetadata{Name: "beta-actions"},
		Components: []v1beta1.Component{{
			Name: "actions",
			ComponentSpec: v1beta1.ComponentSpec{Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{
				Before:    []v1beta1.ComponentAction{{Cmd: "printf from-before", SetValues: []v1beta1.SetValue{{Key: ".output", Type: v1beta1.SetValueString}}}},
				OnSuccess: []v1beta1.ComponentAction{{Cmd: "test '{{ .Values.output }}' = from-before", EnableTemplating: true}},
			}}},
		}},
	}
	b, err := goyaml.Marshal(pkg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), b, 0o600))

	require.NoError(t, DevDeploy(ctx, packageDir, DevDeployOptions{CachePath: filepath.Join(packageDir, "cache")}))
}

func TestDevDeploy_runsV1Beta1FailureActions(t *testing.T) {
	ctx := testutil.TestContext(t)
	packageDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "failure-ran")
	pkg := v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata:   v1beta1.PackageMetadata{Name: "beta-failure-actions"},
		Components: []v1beta1.Component{{
			Name: "actions",
			ComponentSpec: v1beta1.ComponentSpec{Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{
				Before:    []v1beta1.ComponentAction{{Cmd: "false"}},
				OnFailure: []v1beta1.ComponentAction{{Cmd: "touch " + marker}},
			}}},
		}},
	}
	b, err := goyaml.Marshal(pkg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), b, 0o600))

	require.Error(t, DevDeploy(ctx, packageDir, DevDeployOptions{CachePath: filepath.Join(packageDir, "cache")}))
	require.FileExists(t, marker)
}
