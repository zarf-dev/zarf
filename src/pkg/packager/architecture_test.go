// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

// The same imports exercise package target selection and standalone variant selection.
func architectureDefinitions(t *testing.T) (string, string, string) {
	t.Helper()
	target := "arm64"
	if runtime.GOARCH == target {
		target = "amd64"
	}
	dir := t.TempDir()
	imports := fmt.Sprintf("    local:\n      - path: %s.yaml\n      - path: %s.yaml\n", runtime.GOARCH, target)
	for _, arch := range []string{runtime.GOARCH, target} {
		component := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: selected
variant:
  architecture: %s
values:
  files:
    - %s-values.yaml
component:
  images:
    - name: example.com/%s:1
      id: selected
      source: registry
  manifests:
    - name: selected
      files:
        - %s-manifest.yaml
`, arch, arch, arch, arch)
		require.NoError(t, os.WriteFile(filepath.Join(dir, arch+".yaml"), []byte(component), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, arch+"-values.yaml"), []byte(arch+": true\n"), 0o600))
		manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: selected
spec:
  containers:
    - name: selected
      image: example.com/%s:1
`, arch)
		require.NoError(t, os.WriteFile(filepath.Join(dir, arch+"-manifest.yaml"), []byte(manifest), 0o600))
	}
	pkg := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: architecture
  version: 0.0.1
  architecture: %s
components:
  - name: selected
    import:
      local:
        - path: %s.yaml
        - path: %s.yaml
`, runtime.GOARCH, runtime.GOARCH, target)
	require.NoError(t, os.WriteFile(filepath.Join(dir, layout.ZarfYAML), []byte(pkg), 0o600))
	standalone := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: selected
variant:
  architecture: %s
component:
  import:
%s`, runtime.GOARCH, imports)
	componentPath := filepath.Join(dir, "standalone.yaml")
	require.NoError(t, os.WriteFile(componentPath, []byte(standalone), 0o600))
	return dir, componentPath, target
}

func TestDefinitionOperationsArchitecture(t *testing.T) {
	ctx := testutil.TestContext(t)
	pkg, standalone, target := architectureDefinitions(t)
	for _, tt := range []struct{ name, source, want string }{
		{"package override", pkg, target},
		{"standalone variant", standalone, runtime.GOARCH},
	} {
		t.Run(tt.name, func(t *testing.T) {
			image := "example.com/" + tt.want + ":1"
			t.Run("lint", func(t *testing.T) {
				err := Lint(ctx, tt.source, LintOptions{Architecture: target, CachePath: t.TempDir()})
				var findings *lint.LintError
				require.ErrorAs(t, err, &findings)
				require.Len(t, findings.Findings, 1)
				require.Equal(t, image, findings.Findings[0].Item)
			})
			t.Run("inspect resources", func(t *testing.T) {
				resources, err := InspectDefinitionResources(ctx, tt.source, InspectDefinitionResourcesOptions{Architecture: target, CachePath: t.TempDir()})
				require.NoError(t, err)
				require.Len(t, resources, 1)
				require.Contains(t, resources[0].Content, image)
			})
			t.Run("find images", func(t *testing.T) {
				opts := FindImagesOptions{Architecture: target, SkipCosign: true, CachePath: t.TempDir()}
				scans, err := FindImages(ctx, tt.source, opts)
				require.NoError(t, err)
				require.Len(t, scans, 1)
				require.Equal(t, []string{image}, imageMatchNames(scans[0].Matches))
			})
			t.Run("find definition images", func(t *testing.T) {
				opts := FindImagesOptions{Architecture: target, SkipCosign: true, CachePath: t.TempDir()}
				definitions, err := FindDefinitionImages(ctx, tt.source, opts)
				require.NoError(t, err)
				require.Len(t, definitions, 1)
				require.Equal(t, []string{image}, imageMatchNames(definitions[0].Matches))
				require.Equal(t, "selected", definitions[0].Matches[0].Image.ID)
				require.EqualValues(t, "registry", definitions[0].Matches[0].Image.Source)
			})
			t.Run("generate values schema", func(t *testing.T) {
				schema, err := GenerateValuesSchema(ctx, tt.source, GenerateValuesSchemaOptions{DefinitionOptions: load.DefinitionOptions{Architecture: target, CachePath: t.TempDir()}})
				require.NoError(t, err)
				properties, ok := schema["properties"].(map[string]any)
				require.True(t, ok)
				require.Len(t, properties, 1)
				require.Contains(t, properties, tt.want)
			})
		})
	}
}

func TestPublishSkeletonArchitecture(t *testing.T) {
	ctx := testutil.TestContext(t)
	target := "arm64"
	if runtime.GOARCH == target {
		target = "amd64"
	}
	pkg := t.TempDir()
	root := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: architecture
  version: 0.0.1
  architecture: %s
components:
  - name: selected
    import:
      path: leaf
`, runtime.GOARCH)
	require.NoError(t, os.WriteFile(filepath.Join(pkg, layout.ZarfYAML), []byte(root), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(pkg, "leaf"), 0o700))
	leaf := "kind: ZarfPackageConfig\nmetadata:\n  name: leaf\ncomponents:\n"
	for _, arch := range []string{runtime.GOARCH, target} {
		leaf += fmt.Sprintf(`  - name: selected
    only:
      cluster:
        architecture: %s
    images:
      - example.com/%s:1
`, arch, arch)
	}
	require.NoError(t, os.WriteFile(filepath.Join(pkg, "leaf", layout.ZarfYAML), []byte(leaf), 0o600))
	reg := createRegistry(ctx, t)
	ref, err := PublishSkeleton(ctx, pkg, reg, PublishSkeletonOptions{Architecture: target, CachePath: t.TempDir(), RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)
	remote, err := zoci.NewRemoteWithOptions(ctx, ref.String(), zoci.PlatformForSkeleton(), zoci.RemoteClientOptions{RemoteOptions: defaultTestRemoteOptions()})
	require.NoError(t, err)
	definition, err := remote.FetchZarfYAML(ctx)
	require.NoError(t, err)
	require.Equal(t, "skeleton", definition.Metadata.Architecture)
	require.Len(t, definition.Components, 1)
	require.Equal(t, "example.com/"+target+":1", definition.Components[0].Images[0].Name)
}
