// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"k8s.io/client-go/kubernetes/fake"
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
			source: filepath.Join("testdata", "load-package", "compressed", "zarf-package-test-amd64-0.0.1.tar.zst"),
			shasum: "f9b15b1bc0f760a87bad68196b339a8ce8330e3a0241191a826a8962a88061f1",
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
				pkgLayout, err := LoadPackage(ctx, opt)
				require.NoError(t, err)

				require.Equal(t, "test", pkgLayout.Pkg.Metadata.Name)
				require.Equal(t, "0.0.1", pkgLayout.Pkg.Metadata.Version)
				require.Len(t, pkgLayout.Pkg.Components, 1)
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

func TestLoadSplitPackage(t *testing.T) {
	t.Parallel()
	lint.ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")

	ctx := testutil.TestContext(t)

	tests := []struct {
		name        string
		packagePath string
	}{
		{
			name:        "split file output",
			packagePath: filepath.Join("testdata", "load-package", "split"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpdir := t.TempDir()

			f, err := os.Create(filepath.Join(tt.packagePath, "random_1mb.bin"))
			require.NoError(t, err)
			t.Cleanup(func() {
				f.Close()
				require.NoError(t, os.RemoveAll(f.Name()))
			})
			var mb int64 = 1024 * 1024
			_, err = io.CopyN(f, rand.Reader, mb)
			require.NoError(t, err)
			err = Create(ctx, tt.packagePath, CreateOptions{
				Output:           tmpdir,
				MaxPackageSizeMB: 1,
				SkipSBOM:         true,
			})
			require.NoError(t, err)

			splitName := fmt.Sprintf("zarf-package-split-%s.tar.zst.part000", runtime.GOARCH)
			name := filepath.Join(tmpdir, splitName)
			opt := LoadOptions{
				Source:                  name,
				PublicKeyPath:           "",
				SkipSignatureValidation: false,
				Filter:                  filters.Empty(),
			}
			_, err = LoadPackage(ctx, opt)
			require.NoError(t, err)
			assembledName := fmt.Sprintf("zarf-package-split-%s.tar.zst", runtime.GOARCH)
			require.FileExists(t, filepath.Join(tmpdir, assembledName))
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

	_, err := GetPackageFromSourceOrCluster(ctx, nil, "test", false, "", zoci.AllLayers)
	require.EqualError(t, err, "cannot get Zarf package from Kubernetes without configuration")

	pkgPath := filepath.Join("testdata", "load-package", "compressed", "zarf-package-test-amd64-0.0.1.tar.zst")
	pkg, err := GetPackageFromSourceOrCluster(ctx, nil, pkgPath, false, "", zoci.AllLayers)
	require.NoError(t, err)
	require.Equal(t, "test", pkg.Metadata.Name)

	c := &cluster.Cluster{
		Clientset: fake.NewClientset(),
	}
	_, err = c.RecordPackageDeployment(ctx, pkg, nil, 1)
	require.NoError(t, err)
	pkg, err = GetPackageFromSourceOrCluster(ctx, c, "test", false, "", zoci.AllLayers)
	require.NoError(t, err)
	require.Equal(t, "test", pkg.Metadata.Name)
}
