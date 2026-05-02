// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry"

	"github.com/zarf-dev/zarf/src/pkg/packager"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

// TestResolveImportsOCISkeletonValues exercises the OCI skeleton import path: a parent
// package importing a values-bearing skeleton via oci:// must materialize the skeleton's
// merged values.yaml on disk and rewrite Values.Files to a path the consumer can read.
// The publisher-side merge happens in AssembleSkeleton; this test verifies the importer
// side picks the file up. Schema propagation is intentionally out of scope here — see
// the existing TestResolveImports/schema-parent-* fixtures and follow-up work.
func TestResolveImportsOCISkeletonValues(t *testing.T) {
	ctx := testutil.TestContext(t)

	tmp := t.TempDir()
	cachePath := filepath.Join(tmp, "cache")
	require.NoError(t, os.MkdirAll(cachePath, 0o755))

	// Skeleton: declares package-level values + schema.
	skeletonDir := filepath.Join(tmp, "skeleton")
	require.NoError(t, os.MkdirAll(filepath.Join(skeletonDir, "vals"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skeletonDir, "vals", "values.yaml"),
		[]byte("greeting: hello\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skeletonDir, "vals", "values.schema.json"),
		[]byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","properties":{"greeting":{"type":"string"}}}`+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skeletonDir, "zarf.yaml"), []byte(
		`kind: ZarfPackageConfig
metadata:
  name: skeleton-with-values
  version: 0.0.1
values:
  files:
    - vals/values.yaml
  schema: vals/values.schema.json
components:
  - name: noop
    required: true
`), 0o644))

	// Spin up a real in-memory OCI registry on a free port.
	port, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	registryURL := testutil.SetupInMemoryRegistry(ctx, t, port)
	registryRef := registry.Reference{
		Registry:   registryURL,
		Repository: "test",
	}

	publishedRef, err := packager.PublishSkeleton(ctx, skeletonDir, registryRef, packager.PublishSkeletonOptions{
		CachePath:     cachePath,
		RemoteOptions: types.RemoteOptions{PlainHTTP: true},
	})
	require.NoError(t, err)

	// Parent: imports the skeleton via oci://. Defines its own additional values file so we
	// can also confirm parent + imported values end up in the resolved Values.Files list.
	parentDir := filepath.Join(tmp, "parent")
	require.NoError(t, os.MkdirAll(parentDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "parent-values.yaml"),
		[]byte("override: parent\n"), 0o644))
	parentYAML := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: parent
  version: 0.0.1
values:
  files:
    - parent-values.yaml
components:
  - name: noop
    required: true
    import:
      url: oci://%s
`, publishedRef.String())
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "zarf.yaml"), []byte(parentYAML), 0o644))

	resolved, err := load.PackageDefinition(ctx, parentDir, load.DefinitionOptions{
		CachePath:     cachePath,
		RemoteOptions: types.RemoteOptions{PlainHTTP: true},
	})
	require.NoError(t, err)

	// The imported skeleton's merged values must come first (deepest-first), then the parent's.
	require.Len(t, resolved.Values.Files, 2,
		"expected one values file from imported skeleton plus one from parent, got %v", resolved.Values.Files)
	require.Equal(t, "parent-values.yaml", resolved.Values.Files[1])

	// Imported values path must resolve to a real file on disk relative to the parent's BaseDir.
	importedRel := resolved.Values.Files[0]
	importedAbs := filepath.Join(parentDir, importedRel)
	require.FileExists(t, importedAbs,
		"expected imported skeleton values to materialize on disk at %s", importedAbs)

	// Schema propagation across imports is left for future work (path imports also
	// don't propagate today — see TestResolveImports/schema-parent-empty). Parent
	// declares none, so resolved schema stays empty.
	require.Empty(t, resolved.Values.Schema,
		"schema propagation is not part of this fix; parent declares none, so resolved schema must stay empty")
}
