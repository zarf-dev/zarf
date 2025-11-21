// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/registry"
)

func TestAnnotationsFromMetadata(t *testing.T) {
	t.Parallel()

	metadata := v1alpha1.ZarfMetadata{
		Name:          "foo",
		Description:   "bar",
		URL:           "https://example.com",
		Authors:       "Zarf",
		Documentation: "documentation",
		Source:        "source",
		Vendor:        "vendor",
		Annotations: map[string]string{
			"org.opencontainers.image.title": "overridden",
			"org.opencontainers.image.new":   "new-field",
		},
	}
	annotations := annotationsFromMetadata(metadata)
	expectedAnnotations := map[string]string{
		"org.opencontainers.image.title":         "overridden",
		"org.opencontainers.image.description":   "bar",
		"org.opencontainers.image.url":           "https://example.com",
		"org.opencontainers.image.authors":       "Zarf",
		"org.opencontainers.image.documentation": "documentation",
		"org.opencontainers.image.source":        "source",
		"org.opencontainers.image.vendor":        "vendor",
		"org.opencontainers.image.new":           "new-field",
	}
	require.Equal(t, expectedAnnotations, annotations)
}

// TestPushPackageWithDirectoryNameCollision tests that publishing a package
// succeeds even when a directory with the same name as the package metadata
// exists in the current working directory (issue #4148).
func TestPushPackageWithDirectoryNameCollision(t *testing.T) {

	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")
	ctx := testutil.TestContext(t)

	// Setup destination registry
	port, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	registryURL := testutil.SetupInMemoryRegistry(ctx, t, port)
	registryRef := registry.Reference{
		Registry:   registryURL,
		Repository: "test-namespace",
	}

	// Create a minimal package layout in a temp directory
	tmpdir := t.TempDir()
	pkgDir := filepath.Join(tmpdir, "pkg")
	err = os.MkdirAll(pkgDir, 0o755)
	require.NoError(t, err)

	// Create checksums.txt (empty is valid when there are no files to checksum)
	err = os.WriteFile(filepath.Join(pkgDir, layout.Checksums), []byte(""), 0o644)
	require.NoError(t, err)

	// Calculate the SHA256 of the empty checksums.txt file
	// Empty file SHA256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	emptyFileSHA256 := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Create a minimal zarf.yaml with the correct aggregateChecksum
	zarfYAML := `kind: ZarfPackageConfig
metadata:
  name: test-collision-pkg
  version: 0.0.1
  architecture: amd64
  aggregateChecksum: "` + emptyFileSHA256 + `"
build:
  timestamp: "` + time.Now().Format(v1alpha1.BuildTimestampFormat) + `"
components: []
`
	err = os.WriteFile(filepath.Join(pkgDir, layout.ZarfYAML), []byte(zarfYAML), 0o644)
	require.NoError(t, err)

	// Load the package layout
	pkgLayout, err := layout.LoadFromDir(ctx, pkgDir, layout.PackageLayoutOptions{
		Filter: filters.Empty(),
	})
	require.NoError(t, err)

	// Change to tmpdir to reproduce the issue
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		err := os.Chdir(originalWd)
		require.NoError(t, err)
	}()
	err = os.Chdir(tmpdir)
	require.NoError(t, err)

	// Create a directory with the same name as the package metadata
	// This is the condition that triggers issue #4148
	collisionDirName := pkgLayout.Pkg.Metadata.Name
	err = os.Mkdir(collisionDirName, 0o755)
	require.NoError(t, err)

	// Create a Remote instance
	platform := oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture)
	remote, err := NewRemote(ctx, registryRef.String()+"/test-collision-pkg:0.0.1", platform, oci.WithPlainHTTP(true))
	require.NoError(t, err)

	// Try to push the package - this should succeed despite the directory name collision
	opts := PublishOptions{
		OCIConcurrency: 3,
		Retries:        1,
	}

	// This should not fail with "failed to create file <pkg-name>: open <pkg-name>: is a directory"
	_, err = remote.PushPackage(ctx, pkgLayout, opts)
	require.NoError(t, err, "Publishing should succeed even when a directory with the package name exists")
}
