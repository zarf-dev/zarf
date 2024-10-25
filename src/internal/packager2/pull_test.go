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
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
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
	shasum := "bef73d652f004d214d5cf9e00195293f7ae8390b8ff6ed45e39c2c9eb622b873"
	err := Pull(ctx, srv.URL, dir, shasum, filters.Empty())
	require.NoError(t, err)

	packageData, err := os.ReadFile(packagePath)
	require.NoError(t, err)
	pulledPath := filepath.Join(dir, "zarf-package-test-amd64-0.0.1.tar.zst")
	pulledData, err := os.ReadFile(pulledPath)
	require.NoError(t, err)
	require.Equal(t, packageData, pulledData)
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
