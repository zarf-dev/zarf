// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestLoadPackage(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	tests := []struct {
		name   string
		source string
		shasum string
	}{
		{
			name:   "tarball",
			source: "./testdata/zarf-package-test-amd64-0.0.1.tar.zst",
			shasum: "307294e3a066cebea6f04772c2ba31210b2753b40b0d5da86a1983c29c5545dd",
		},
		{
			name:   "split",
			source: "./testdata/zarf-package-test-amd64-0.0.1.tar.zst.part000",
			shasum: "6c0de217e3eeff224679ec0a26751655759a30f4aae7fbe793ca1617ddfc4228",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, shasum := range []string{tt.shasum, ""} {
				opt := LoadOptions{
					Source:                  tt.source,
					Shasum:                  shasum,
					PublicKeyPath:           "",
					SkipSignatureValidation: false,
					Filter:                  filters.Empty(),
				}
				pkgPaths, err := LoadPackage(ctx, opt)
				require.NoError(t, err)

				pkg, _, err := pkgPaths.ReadZarfYAML()
				require.NoError(t, err)
				require.Equal(t, "test", pkg.Metadata.Name)
				require.Equal(t, "0.0.1", pkg.Metadata.Version)
				require.Len(t, pkg.Components, 1)
			}

			opt := LoadOptions{
				Source:                  tt.source,
				Shasum:                  "foo",
				PublicKeyPath:           "",
				SkipSignatureValidation: false,
				Filter:                  filters.Empty(),
			}
			_, err := LoadPackage(ctx, opt)
			require.ErrorContains(t, err, fmt.Sprintf("to be %s, found %s", opt.Shasum, tt.shasum))
		})
	}
}

func TestIdentifySource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		src             string
		expectedSrcType string
	}{
		{
			name:            "oci",
			src:             "oci://ghcr.io/defenseunicorns/packages/init:1.0.0",
			expectedSrcType: "oci",
		},
		{
			name:            "sget with sub path",
			src:             "sget://github.com/defenseunicorns/zarf-hello-world:x86",
			expectedSrcType: "sget",
		},
		{
			name:            "sget without host",
			src:             "sget://defenseunicorns/zarf-hello-world:x86_64",
			expectedSrcType: "sget",
		},
		{
			name:            "https",
			src:             "https://github.com/zarf-dev/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst",
			expectedSrcType: "https",
		},
		{
			name:            "http",
			src:             "http://github.com/zarf-dev/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst",
			expectedSrcType: "http",
		},
		{
			name:            "local tar init zst",
			src:             "zarf-init-amd64-v1.0.0.tar.zst",
			expectedSrcType: "tarball",
		},
		{
			name:            "local tar",
			src:             "zarf-package-manifests-amd64-v1.0.0.tar",
			expectedSrcType: "tarball",
		},
		{
			name:            "local tar manifest zst",
			src:             "zarf-package-manifests-amd64-v1.0.0.tar.zst",
			expectedSrcType: "tarball",
		},
		{
			name:            "local tar split",
			src:             "testdata/.part000",
			expectedSrcType: "split",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srcType, err := identifySource(tt.src)
			require.NoError(t, err)
			require.Equal(t, tt.expectedSrcType, srcType)
		})
	}
}

func TestPackageFromSourceOrCluster(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)

	_, err := packageFromSourceOrCluster(ctx, nil, "test", false)
	require.EqualError(t, err, "cannot get Zarf package from Kubernetes without configuration")

	pkg, err := packageFromSourceOrCluster(ctx, nil, "./testdata/zarf-package-test-amd64-0.0.1.tar.zst", false)
	require.NoError(t, err)
	require.Equal(t, "test", pkg.Metadata.Name)

	c := &cluster.Cluster{
		Clientset: fake.NewSimpleClientset(),
	}
	_, err = c.RecordPackageDeployment(ctx, pkg, nil, 1)
	require.NoError(t, err)
	// pkg, err = packageFromSourceOrCluster(ctx, c, "test", false)
	// require.NoError(t, err)
	// require.Equal(t, "test", pkg.Metadata.Name)
}
