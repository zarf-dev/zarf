// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestMain(m *testing.M) {
	if err := feature.Set([]feature.Feature{{Name: feature.Values, Enabled: true}}); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestValuesBearingSkeletonOCIImport(t *testing.T) {
	ctx := testutil.TestContext(t)
	cachePath := filepath.Join(t.TempDir(), "cache")
	skeletonPath := filepath.Join(t.TempDir(), "skeleton")
	require.NoError(t, os.MkdirAll(skeletonPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(skeletonPath, "values.yaml"), []byte("shared: 1\nchild: value\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(skeletonPath, "values.schema.json"), []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","properties":{"shared":{"type":"integer"},"child":{"type":"string"}}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(skeletonPath, layout.ZarfYAML), []byte(`kind: ZarfPackageConfig
metadata:
  name: values-skeleton
  version: 0.0.1
values:
  files:
    - values.yaml
  schema: values.schema.json
components:
  - name: first
  - name: second
`), 0o600))

	skeletonRef, err := PublishSkeleton(ctx, skeletonPath, createRegistry(ctx, t), PublishSkeletonOptions{
		CachePath:     cachePath,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	parentPath := filepath.Join(t.TempDir(), "parent")
	require.NoError(t, os.MkdirAll(parentPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(parentPath, "values.yaml"), []byte("shared: parent\nparent: value\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(parentPath, "values.schema.json"), []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","properties":{"shared":{"type":"string"},"parent":{"type":"string"}}}`), 0o600))
	parentManifest := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: values-parent
  version: 0.0.1
values:
  files:
    - values.yaml
  schema: values.schema.json
components:
  - name: imported-first
    import:
      url: oci://%s
      name: first
  - name: imported-second
    import:
      url: oci://%s
      name: second
`, skeletonRef.String(), skeletonRef.String())
	require.NoError(t, os.WriteFile(filepath.Join(parentPath, layout.ZarfYAML), []byte(parentManifest), 0o600))

	_, err = load.PackageDefinition(ctx, parentPath, load.DefinitionOptions{
		CachePath:     cachePath,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.ErrorContains(t, err, "a temporary directory is required to stage values from imported skeleton")

	tmpDir := t.TempDir()
	defined, err := load.PackageDefinition(ctx, parentPath, load.DefinitionOptions{
		CachePath:     cachePath,
		TempDir:       tmpDir,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)
	require.Len(t, defined.Pkg.Values.Files, 2, "repeated imports must contribute values once")
	require.Len(t, defined.ImportedSchemas, 1, "repeated imports must contribute schemas once")
	require.FileExists(t, filepath.Join(parentPath, defined.Pkg.Values.Files[0]))
	require.FileExists(t, filepath.Join(parentPath, defined.ImportedSchemas[0]))
	stagedValuesPath := filepath.Join(parentPath, defined.Pkg.Values.Files[0])
	relativeStagedPath, err := filepath.Rel(tmpDir, stagedValuesPath)
	require.NoError(t, err)
	require.False(t, strings.HasPrefix(relativeStagedPath, ".."), "values must be staged in the caller-provided workspace")

	pkgLayout, err := assemble.AssemblePackage(ctx, defined.Pkg, parentPath, defined.ImportedSchemas, assemble.AssembleOptions{
		CachePath:     cachePath,
		SkipSBOM:      true,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)
	values, err := os.ReadFile(filepath.Join(pkgLayout.DirPath(), layout.ValuesYAML))
	require.NoError(t, err)
	require.YAMLEq(t, "shared: parent\nchild: value\nparent: value\n", string(values))

	schemaBytes, err := os.ReadFile(filepath.Join(pkgLayout.DirPath(), layout.ValuesSchema))
	require.NoError(t, err)
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(schemaBytes, &schema))
	require.Equal(t, "string", schema.Properties["shared"].Type, "parent schema must take precedence")
	require.Equal(t, "string", schema.Properties["child"].Type)
	require.Equal(t, "string", schema.Properties["parent"].Type)
}
