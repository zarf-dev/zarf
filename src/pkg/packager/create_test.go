// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
