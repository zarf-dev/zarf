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

func TestPackageCreatePreservesV1alpha1MetadataAnnotationCollisions(t *testing.T) {
	ctx := testutil.TestContext(t)
	packageDir := t.TempDir()
	definition := `kind: ZarfPackageConfig
metadata:
  name: metadata-collisions
  architecture: amd64
  url: field-url
  image: field-image
  authors: field-authors
  documentation: field-documentation
  source: field-source
  vendor: field-vendor
  annotations:
    url: annotation-url
    image: annotation-image
    authors: annotation-authors
    documentation: annotation-documentation
    source: annotation-source
    vendor: annotation-vendor
components:
- name: empty
`
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), []byte(definition), 0o600))

	packagePath, err := Create(ctx, packageDir, t.TempDir(), CreateOptions{
		CachePath: t.TempDir(),
		SkipSBOM:  true,
	})
	require.NoError(t, err)

	pkgLayout, err := layout.LoadFromTar(ctx, packagePath, layout.PackageLayoutOptions{VerificationStrategy: layout.VerifyNever})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	metadata := pkgLayout.Definition().Metadata
	require.Equal(t, "field-url", metadata.URL)
	require.Equal(t, "field-image", metadata.Image)
	require.Equal(t, "field-authors", metadata.Authors)
	require.Equal(t, "field-documentation", metadata.Documentation)
	require.Equal(t, "field-source", metadata.Source)
	require.Equal(t, "field-vendor", metadata.Vendor)
	require.Equal(t, map[string]string{
		"url":           "annotation-url",
		"image":         "annotation-image",
		"authors":       "annotation-authors",
		"documentation": "annotation-documentation",
		"source":        "annotation-source",
		"vendor":        "annotation-vendor",
	}, metadata.Annotations)
}

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
			layout := pullFromRemote(ctx, t, packageSource, tt.expectedArch, "", t.TempDir(), defaultTestRemoteOptions())
			require.Equal(t, tt.expectedArch, layout.Definition().Metadata.Architecture)
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
