// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
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

func TestPackageCreateOCITransportNegotiation(t *testing.T) {
	const (
		username = "registry-user"
		password = "registry-password"
	)

	ctx := testutil.TestContext(t)
	registryAddress := testutil.SetupInMemoryRegistryTLSAuth(ctx, t, username, password)
	setDockerConfig(t, map[string]bool{registryAddress: true}, username, password)

	packageSource, err := Create(ctx, filepath.Join("testdata", "create", "create-publish-arch"), fmt.Sprintf("oci://%s/my-namespace", registryAddress), CreateOptions{
		RemoteOptions: types.RemoteOptions{
			PlainHTTP:             true,
			InsecureSkipTLSVerify: true,
		},
	})
	require.NoError(t, err)

	layout := pullFromRemoteWithOptions(ctx, t, packageSource, "amd64", "", t.TempDir(), types.RemoteOptions{
		InsecureSkipTLSVerify: true,
	})
	require.Equal(t, "amd64", layout.Pkg.Metadata.Architecture)
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
