// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

const testTarball = "testdata/load-package/compressed/zarf-package-test-amd64-0.0.1.tar.zst"

func TestPackageDigestTarball(t *testing.T) {
	ctx := testutil.TestContext(t)

	digest, err := PackageDigest(ctx, testTarball, PackageDigestOptions{})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(digest, "sha256:"), "digest should start with sha256:")
	require.Len(t, digest, 71, "sha256 digest string should be 7 (prefix) + 64 (hex) chars")
}

// TestPackageDigestDeterministic pins the exact OCI manifest digest for the test
// package tarball. If this test fails after a code change it means the digest
// computation shifted, which would cause packages deployed with an older Zarf
// version to no longer match the digest stored in cluster state.
func TestPackageDigestDeterministic(t *testing.T) {
	const expectedDigest = "sha256:02b4fefb7469b1c156894198e44f804b647ffdc38e160624023d4897eb78401f"

	ctx := testutil.TestContext(t)

	digest, err := PackageDigest(ctx, testTarball, PackageDigestOptions{})
	require.NoError(t, err)
	require.Equal(t, expectedDigest, digest,
		"digest changed — this breaks cluster digest tracking for packages deployed with older Zarf versions; update expectedDigest only after confirming the change is intentional")
}

func TestPackageDigestOCI(t *testing.T) {
	const expectedDigest = "sha256:02b4fefb7469b1c156894198e44f804b647ffdc38e160624023d4897eb78401f"

	ctx := testutil.TestContext(t)
	registryRef := createRegistry(ctx, t)

	pkgLayout, err := layout.LoadFromTar(ctx, testTarball, layout.PackageLayoutOptions{Filter: filters.Empty()})
	require.NoError(t, err)

	packageRef, err := PublishPackage(ctx, pkgLayout, registryRef, PublishPackageOptions{
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	// PackageDigest expects an oci:// URL so identifySource classifies it correctly.
	ociURL := fmt.Sprintf("oci://%s", packageRef.String())
	digest, err := PackageDigest(ctx, ociURL, PackageDigestOptions{
		Architecture:  pkgLayout.Pkg.Build.Architecture,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)
	require.Equal(t, expectedDigest, digest, "the OCI digest should match the expected digest after publishing and lookup with PackageDigest")
}

func testSignOpts() signing.SignBlobOptions {
	opts := signing.DefaultSignBlobOptions()
	opts.Key = filepath.Join("testdata", "publish", "cosign.key")
	opts.Password = "password"
	return opts
}

// TestPackageDigestSignedDiffersFromUnsigned verifies that the sig file is
// actually included in the manifest after signing — a regression where signing
// no longer updated the digest would silently break cluster state tracking.
func TestPackageDigestSignedDiffersFromUnsigned(t *testing.T) {
	ctx := testutil.TestContext(t)

	unsignedDigest, err := PackageDigest(ctx, testTarball, PackageDigestOptions{})
	require.NoError(t, err)

	pkgLayout, err := layout.LoadFromTar(ctx, testTarball, layout.PackageLayoutOptions{Filter: filters.Empty()})
	require.NoError(t, err)
	defer func() { require.NoError(t, pkgLayout.Cleanup()) }()

	require.NoError(t, pkgLayout.SignPackage(ctx, testSignOpts()))

	require.NotEqual(t, unsignedDigest, pkgLayout.Digest(),
		"signing should change the OCI manifest digest because the sig file is a new layer")
}

// TestPackageDigestSignedConsistency verifies that the locally computed digest
// for a signed package matches the manifest digest resolved from the registry.
// This catches regressions where computeManifest mishandles provenance files.
func TestPackageDigestSignedConsistency(t *testing.T) {
	ctx := testutil.TestContext(t)
	registryRef := createRegistry(ctx, t)

	pkgLayout, err := layout.LoadFromTar(ctx, testTarball, layout.PackageLayoutOptions{Filter: filters.Empty()})
	require.NoError(t, err)
	defer func() { require.NoError(t, pkgLayout.Cleanup()) }()

	require.NoError(t, pkgLayout.SignPackage(ctx, testSignOpts()))
	localDigest := pkgLayout.Digest()

	packageRef, err := PublishPackage(ctx, pkgLayout, registryRef, PublishPackageOptions{
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)

	ociURL := fmt.Sprintf("oci://%s", packageRef.String())
	ociDigest, err := PackageDigest(ctx, ociURL, PackageDigestOptions{
		Architecture:  pkgLayout.Pkg.Build.Architecture,
		RemoteOptions: defaultTestRemoteOptions(),
	})
	require.NoError(t, err)
	require.Equal(t, localDigest, ociDigest,
		"locally computed signed digest must match the manifest digest resolved from the OCI registry")
}

func TestPackageDigestErrors(t *testing.T) {
	ctx := context.Background()

	tt := []struct {
		name      string
		source    string
		opts      PackageDigestOptions
		expectErr string
	}{
		{
			name:      "unknown source type",
			source:    "/nonexistent/path/to/package",
			expectErr: "unknown source",
		},
		{
			name:      "cluster source without cluster connection",
			source:    "my-package",
			opts:      PackageDigestOptions{Cluster: nil},
			expectErr: "cluster connection is required",
		},
		{
			name:      "split tarball from missing file",
			source:    filepath.Join("testdata", "nonexistent.part000"),
			expectErr: "unable to reassemble",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PackageDigest(ctx, tc.source, tc.opts)
			require.ErrorContains(t, err, tc.expectErr)
		})
	}
}
