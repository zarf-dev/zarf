// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

func defaultTestRemoteOptions() RemoteOptions {
	return RemoteOptions{
		PlainHTTP: true,
	}
}

func pullFromRemote(ctx context.Context, t *testing.T, packageRef string, architecture string, publicKeyPath string, cachePath string) *layout.PackageLayout {
	t.Helper()

	// Generate tmpdir and pull published package from local registry
	pullOCIOpts := pullOCIOptions{
		Source:        packageRef,
		Architecture:  architecture,
		Filter:        filters.Empty(),
		RemoteOptions: defaultTestRemoteOptions(),
		PublicKeyPath: publicKeyPath,
		CachePath:     cachePath,
	}
	pkgLayout, err := pullOCI(ctx, pullOCIOpts)
	require.NoError(t, err)

	return pkgLayout
}

func createRegistry(ctx context.Context, t *testing.T) registry.Reference {
	// Setup destination registry
	dstPort, err := helpers.GetAvailablePort()
	require.NoError(t, err)
	dstRegistryURL := testutil.SetupInMemoryRegistry(ctx, t, dstPort)
	dstRegistryRef := registry.Reference{
		Registry:   dstRegistryURL,
		Repository: "my-namespace",
	}

	return dstRegistryRef
}

func TestPublishError(t *testing.T) {
	ctx := context.Background()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")

	registryURL := testutil.SetupInMemoryRegistry(ctx, t, 5000)
	defaultRef := registry.Reference{
		Registry:   registryURL,
		Repository: "my-namespace",
	}

	tt := []struct {
		name          string
		packageLayout *layout.PackageLayout
		ref           registry.Reference
		opts          PublishPackageOptions
		expectErr     error
	}{
		{
			name:      "Test empty publish opts",
			opts:      PublishPackageOptions{},
			expectErr: errors.New("invalid registry"),
		},
		{
			name:          "Test empty path",
			packageLayout: nil,
			ref:           defaultRef,
			opts:          PublishPackageOptions{},
			expectErr:     errors.New("package layout must be specified"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PublishPackage(context.Background(), tc.packageLayout, tc.ref, tc.opts)
			require.ErrorContains(t, err, tc.expectErr.Error())
		})
	}
}

func TestPublishFromOCIValidation(t *testing.T) {
	ctx := context.Background()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")

	tt := []struct {
		name      string
		src       registry.Reference
		dst       registry.Reference
		opts      PublishFromOCIOptions
		expectErr error
	}{
		{
			name: "errors if src is not a valid ref",
			src: registry.Reference{
				Registry:   "example.com",
				Repository: "my-namespace",
			},
			dst:       registry.Reference{},
			opts:      PublishFromOCIOptions{},
			expectErr: errdef.ErrInvalidReference,
		},
		{
			name: "errors if dst is not a valid ref",
			src: registry.Reference{
				Registry:   "example.com",
				Repository: "my-namespace",
			},
			dst:       registry.Reference{},
			opts:      PublishFromOCIOptions{},
			expectErr: errdef.ErrInvalidReference,
		},
		{
			name: "errors if src's repo name is not the same as dst's",
			src: registry.Reference{
				Registry:   "example.com",
				Repository: "my-namespace",
			},
			dst: registry.Reference{
				Registry:   "example.com",
				Repository: "my-other-namespace",
			},
			opts:      PublishFromOCIOptions{},
			expectErr: errors.New("source and destination repositories must have the same name"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := PublishFromOCI(ctx, tc.src, tc.dst, tc.opts)
			if tc.expectErr != nil {
				require.ErrorContains(t, err, tc.expectErr.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPublishSkeleton(t *testing.T) {
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")

	tt := []struct {
		name string
		path string
		opts PublishSkeletonOptions
	}{
		{
			name: "Publish skeleton package",
			path: "testdata/skeleton",
			opts: PublishSkeletonOptions{
				RemoteOptions: defaultTestRemoteOptions(),
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			registryRef := createRegistry(ctx, t)

			// Publish test package
			ref, err := PublishSkeleton(ctx, tc.path, registryRef, tc.opts)
			require.NoError(t, err)

			// Read and unmarshall expected
			data, err := os.ReadFile(filepath.Join(tc.path, layout.ZarfYAML))
			require.NoError(t, err)
			var expectedPkg v1alpha1.ZarfPackage
			err = goyaml.Unmarshal(data, &expectedPkg)
			require.NoError(t, err)
			// This verifies that publish deletes the manifest that is auto created by oras
			require.NoFileExists(t, expectedPkg.Metadata.Name)

			rmt, err := zoci.NewRemote(ctx, ref.String(), zoci.PlatformForSkeleton(), oci.WithPlainHTTP(true))
			require.NoError(t, err)

			// Fetch from remote and compare
			pkg, err := rmt.FetchZarfYAML(ctx)
			require.NoError(t, err)

			// HACK(mkcp): Match necessary fields to establish equality
			pkg.Build = v1alpha1.ZarfBuildData{}
			pkg.Metadata.AggregateChecksum = ""
			expectedPkg.Metadata.Architecture = "skeleton"

			// NOTE(mkcp): In future schema version move ZarfPackage.Metadata.AggregateChecksum
			// to ZarfPackage.Build.AggregateChecksum. See ADR #26
			require.Equal(t, expectedPkg, pkg)
		})
	}
}

func TestPublishPackage(t *testing.T) {
	tt := []struct {
		name          string
		path          string
		opts          PublishPackageOptions
		publicKeyPath string
	}{
		{
			name: "Publish package",
			path: filepath.Join("testdata", "load-package", "compressed", "zarf-package-test-amd64-0.0.1.tar.zst"),
			opts: PublishPackageOptions{
				RemoteOptions: defaultTestRemoteOptions(),
			},
		},
		{
			name: "Sign and publish package",
			path: filepath.Join("testdata", "load-package", "compressed", "zarf-package-test-amd64-0.0.1.tar.zst"),
			opts: PublishPackageOptions{
				RemoteOptions:      defaultTestRemoteOptions(),
				SigningKeyPath:     filepath.Join("testdata", "publish", "cosign.key"),
				SigningKeyPassword: "password",
			},
			publicKeyPath: filepath.Join("testdata", "publish", "cosign.pub"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			registryRef := createRegistry(ctx, t)

			// We want to pull the package and sure the content is the same as the local package
			layoutExpected, err := layout.LoadFromTar(ctx, tc.path, layout.PackageLayoutOptions{Filter: filters.Empty()})
			require.NoError(t, err)

			// Publish test package
			packageRef, err := PublishPackage(ctx, layoutExpected, registryRef, tc.opts)
			require.NoError(t, err)

			layoutActual := pullFromRemote(ctx, t, packageRef.String(), "amd64", tc.publicKeyPath, t.TempDir())
			require.Equal(t, layoutExpected.Pkg, layoutActual.Pkg, "Uploaded package is not identical to downloaded package")
			if tc.opts.SigningKeyPath != "" {
				require.FileExists(t, filepath.Join(layoutActual.DirPath(), layout.Signature))
			}
		})
	}
}

func TestPublishPackageDeterministic(t *testing.T) {
	tt := []struct {
		name string
		path string
		opts PublishPackageOptions
	}{
		{
			name: "Publish package",
			path: filepath.Join("testdata", "load-package", "compressed", "zarf-package-test-amd64-0.0.1.tar.zst"),
			opts: PublishPackageOptions{
				RemoteOptions: defaultTestRemoteOptions(),
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			registryRef := createRegistry(ctx, t)

			// We want to pull the package and sure the content is the same as the local package
			layoutExpected, err := layout.LoadFromTar(ctx, tc.path, layout.PackageLayoutOptions{Filter: filters.Empty()})
			require.NoError(t, err)

			// Publish test package
			packageRef, err := PublishPackage(ctx, layoutExpected, registryRef, tc.opts)
			require.NoError(t, err)

			// Attempt to get the digest
			platform := oci.PlatformForArch(layoutExpected.Pkg.Build.Architecture)
			remote, err := zoci.NewRemote(ctx, packageRef.String(), platform, oci.WithPlainHTTP(tc.opts.PlainHTTP))
			require.NoError(t, err)
			desc, err := remote.ResolveRoot(ctx)
			require.NoError(t, err)
			expectedDigest := desc.Digest.String()

			// Re-publish the package to ensure the digest does not change
			_, err = PublishPackage(ctx, layoutExpected, registryRef, tc.opts)
			require.NoError(t, err)
			// Publish creates a local oci manifest file using the package name, which gets deleted
			require.NoFileExists(t, layoutExpected.Pkg.Metadata.Name)

			latestDesc, err := remote.ResolveRoot(ctx)
			require.NoError(t, err)

			require.Equal(t, expectedDigest, latestDesc.Digest.String(), "Original digest is not the same as the latest")
		})
	}
}

func TestPublishCopySHA(t *testing.T) {
	tt := []struct {
		name             string
		packageToPublish string
		opts             PublishPackageOptions
	}{
		{
			name:             "Publish package",
			packageToPublish: filepath.Join("testdata", "load-package", "compressed", "zarf-package-test-amd64-0.0.1.tar.zst"),
			opts: PublishPackageOptions{
				RemoteOptions:  defaultTestRemoteOptions(),
				OCIConcurrency: 3,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			registryRef := createRegistry(ctx, t)

			// We want to pull the package and sure the content is the same as the local package
			layoutExpected, err := layout.LoadFromTar(ctx, tc.packageToPublish, layout.PackageLayoutOptions{})
			require.NoError(t, err)

			// Publish test package
			srcRef, err := PublishPackage(ctx, layoutExpected, registryRef, tc.opts)
			require.NoError(t, err)

			// Setup destination registry
			dstRegistryRef := createRegistry(ctx, t)

			// This gets the test package digest from the first package publish
			localRepo := &remote.Repository{PlainHTTP: true}
			localRepo.Reference = srcRef
			indexDesc, err := oras.Resolve(ctx, localRepo, srcRef.String(), oras.ResolveOptions{})
			require.NoError(t, err)
			src := fmt.Sprintf("%s@%s", srcRef, indexDesc.Digest)
			srcRefWithDigest, err := registry.ParseReference(src)
			require.NoError(t, err)

			dst := fmt.Sprintf("%s/%s", dstRegistryRef.String(), "test:0.0.1")
			dstRef, err := registry.ParseReference(dst)
			require.NoError(t, err)

			opts := PublishFromOCIOptions{
				RemoteOptions:  tc.opts.RemoteOptions,
				Architecture:   layoutExpected.Pkg.Build.Architecture,
				OCIConcurrency: tc.opts.OCIConcurrency,
			}

			// Publish test package to the destination registry
			err = PublishFromOCI(ctx, srcRefWithDigest, dstRef, opts)
			require.NoError(t, err)

			// This verifies that publish deletes the manifest that is auto created by oras
			require.NoFileExists(t, layoutExpected.Pkg.Metadata.Name)

			pkgRefSha := fmt.Sprintf("%s@%s", dstRef.String(), indexDesc.Digest)

			layoutActual := pullFromRemote(ctx, t, pkgRefSha, layoutExpected.Pkg.Build.Architecture, "", t.TempDir())
			require.Equal(t, layoutExpected.Pkg, layoutActual.Pkg, "Uploaded package is not identical to downloaded package")
		})
	}
}

func TestPublishCopyTag(t *testing.T) {
	tt := []struct {
		name             string
		packageToPublish string
		opts             PublishPackageOptions
	}{
		{
			name:             "Publish package",
			packageToPublish: filepath.Join("testdata", "load-package", "compressed", "zarf-package-test-amd64-0.0.1.tar.zst"),
			opts: PublishPackageOptions{
				RemoteOptions:  defaultTestRemoteOptions(),
				OCIConcurrency: 3,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			registryRef := createRegistry(ctx, t)

			// We want to pull the package and sure the content is the same as the local package
			layoutExpected, err := layout.LoadFromTar(ctx, tc.packageToPublish, layout.PackageLayoutOptions{})
			require.NoError(t, err)

			// Publish test package
			srcRef, err := PublishPackage(ctx, layoutExpected, registryRef, tc.opts)
			require.NoError(t, err)

			dstRegistryRef := createRegistry(ctx, t)

			dst := fmt.Sprintf("%s/%s", dstRegistryRef.String(), "test:0.0.1")
			dstRegistry, err := registry.ParseReference(dst)
			require.NoError(t, err)

			opts := PublishFromOCIOptions{
				RemoteOptions:  tc.opts.RemoteOptions,
				Architecture:   layoutExpected.Pkg.Build.Architecture,
				OCIConcurrency: tc.opts.OCIConcurrency,
			}

			// Publish test package
			err = PublishFromOCI(ctx, srcRef, dstRegistry, opts)
			require.NoError(t, err)

			// This verifies that publish deletes the manifest that is auto created by oras
			require.NoFileExists(t, layoutExpected.Pkg.Metadata.Name)

			layoutActual := pullFromRemote(ctx, t, dstRegistry.String(), layoutExpected.Pkg.Build.Architecture, "", t.TempDir())

			require.Equal(t, layoutExpected.Pkg, layoutActual.Pkg, "Uploaded package is not identical to downloaded package")
		})
	}
}
