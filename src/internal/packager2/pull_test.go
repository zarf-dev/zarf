// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPull(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	packagePath := "./testdata/zarf-package-test-amd64-0.0.1.tar.zst"
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		file, err := os.Open(packagePath)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		//nolint:errcheck // ignore
		io.Copy(rw, file)
	}))
	t.Cleanup(func() {
		srv.Close()
	})

	dir := t.TempDir()
	err := Pull(ctx, srv.URL, dir, PullOptions{
		SHASum:       "f9b15b1bc0f760a87bad68196b339a8ce8330e3a0241191a826a8962a88061f1",
		Architecture: "amd64",
	})
	require.NoError(t, err)

	packageData, err := os.ReadFile(packagePath)
	require.NoError(t, err)
	pulledPath := filepath.Join(dir, "zarf-package-test-amd64-0.0.1.tar.zst")
	pulledData, err := os.ReadFile(pulledPath)
	require.NoError(t, err)
	require.Equal(t, packageData, pulledData)
}

func TestPullUncompressed(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	packagePath := "./testdata/uncompressed/zarf-package-test-uncompressed-amd64-0.0.1.tar"
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		file, err := os.Open(packagePath)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		//nolint:errcheck // ignore
		io.Copy(rw, file)
	}))
	t.Cleanup(func() {
		srv.Close()
	})

	dir := t.TempDir()
	err := Pull(ctx, srv.URL, dir, PullOptions{
		SHASum:       "a118a4d306acc5dd4eab2c161e78fa3dfd1e08ae1e1794a4393be98c79257f5c",
		Architecture: "amd64",
	})
	require.NoError(t, err)

	packageData, err := os.ReadFile(packagePath)
	require.NoError(t, err)
	pulledPath := filepath.Join(dir, "zarf-package-test-uncompressed-amd64-0.0.1.tar")
	pulledData, err := os.ReadFile(pulledPath)
	require.NoError(t, err)
	require.Equal(t, packageData, pulledData)
}

func TestPullUnsupported(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	packagePath := "./testdata/uncompressed/zarf.yaml"
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		file, err := os.Open(packagePath)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		//nolint:errcheck // ignore
		io.Copy(rw, file)
	}))
	t.Cleanup(func() {
		srv.Close()
	})

	dir := t.TempDir()
	err := Pull(ctx, srv.URL, dir, PullOptions{
		SHASum:       "6e9dccce07ba9d3c45b7c872fae863c5415d296fd5e2fb72a2583530aa750ccd",
		Architecture: "amd64",
	})
	require.EqualError(t, err, "unsupported file type: .txt", "unsupported file type: .txt")
}

func TestSupportsFiltering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform *ocispec.Platform
		expected bool
	}{
		{
			name:     "nil platform",
			platform: nil,
			expected: false,
		},
		{
			name:     "skeleton platform",
			platform: &ocispec.Platform{OS: oci.MultiOS, Architecture: zoci.SkeletonArch},
			expected: false,
		},
		{
			name:     "linux platform",
			platform: &ocispec.Platform{OS: "linux", Architecture: "amd64"},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := supportsFiltering(tt.platform)
			require.Equal(t, tt.expected, result)
		})
	}
}
