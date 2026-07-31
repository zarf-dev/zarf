// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPackageCreatePublishArch(t *testing.T) {
	ctx := testutil.TestContext(t)
	tests := []struct {
		name         string
		path         string
		expectedArch string
	}{
		{
			name:         "should use pkg.metadata.architecture when global arch not set",
			path:         filepath.Join("testdata", "create", "create-publish-arch"),
			expectedArch: "amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := createRegistry(ctx, t)
			packageSource, err := Create(ctx, tt.path, fmt.Sprintf("oci://%s", reg.String()), CreateOptions{
				RemoteOptions: defaultTestRemoteOptions(),
			})
			require.NoError(t, err)
			layout := pullFromRemote(ctx, t, packageSource, tt.expectedArch, "", t.TempDir())
			require.Equal(t, tt.expectedArch, layout.Pkg.Metadata.Architecture)
		})
	}
}

func TestPackageCreateDifferentialOCIPackage(t *testing.T) {
	ctx := testutil.TestContext(t)
	tests := []struct {
		name           string
		newPackagePath string
		oldPackagePath string
	}{
		{
			name:           "differential package builds from OCI source",
			oldPackagePath: filepath.Join("testdata", "create", "differential", "older-version"),
			newPackagePath: filepath.Join("testdata", "create", "differential"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := createRegistry(ctx, t)
			packageSource, err := Create(ctx, tt.oldPackagePath, fmt.Sprintf("oci://%s", reg.String()), CreateOptions{
				RemoteOptions: defaultTestRemoteOptions(),
			})
			require.NoError(t, err)
			tmpdir := t.TempDir()
			newPackageSource, err := Create(ctx, tt.newPackagePath, tmpdir, CreateOptions{
				DifferentialPackagePath: fmt.Sprintf("oci://%s", packageSource),
				RemoteOptions:           defaultTestRemoteOptions(),
				CachePath:               t.TempDir(),
			})
			require.NoError(t, err)
			require.Equal(t, filepath.Join(tmpdir, "zarf-package-differential-test-amd64-0.0.1-differential-0.0.2.tar.zst"), newPackageSource)
		})
	}
}

func TestPackageCreateWithImportedSkeletonValues(t *testing.T) {
	ctx := testutil.TestContext(t)
	cachePath := filepath.Join(t.TempDir(), "cache")
	registryRef := createRegistry(ctx, t)

	// Publish a skeleton whose values and schema will be imported by the parent package.
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
  - name: imported
`), 0o600))

	skeletonRef, err := PublishSkeleton(ctx, skeletonPath, registryRef, PublishSkeletonOptions{
		CachePath:     cachePath,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	// Create a parent that imports the skeleton component and overrides its values and schema.
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
  - name: imported
    import:
      url: oci://%s
      name: imported
`, skeletonRef.String())
	require.NoError(t, os.WriteFile(filepath.Join(parentPath, layout.ZarfYAML), []byte(parentManifest), 0o600))

	packageRef, err := Create(ctx, parentPath, fmt.Sprintf("oci://%s", registryRef.String()), CreateOptions{
		CachePath:     cachePath,
		SkipSBOM:      true,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	// Pull the created package to verify the assembled values and schema layers.
	created := pullFromRemote(ctx, t, packageRef, "amd64", "", t.TempDir())
	defer func() { require.NoError(t, created.Cleanup()) }()
	values, err := os.ReadFile(filepath.Join(created.DirPath(), layout.ValuesYAML))
	require.NoError(t, err)
	require.YAMLEq(t, "shared: parent\nchild: value\nparent: value\n", string(values))

	schema, err := os.ReadFile(filepath.Join(created.DirPath(), layout.ValuesSchema))
	require.NoError(t, err)
	require.JSONEq(t, `{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","properties":{"shared":{"type":"string"},"child":{"type":"string"},"parent":{"type":"string"}}}`, string(schema))
}
