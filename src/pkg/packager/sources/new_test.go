// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package sources contains core implementations of the PackageSource interface.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/types"
)

func TestNewPackageSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		src              string
		expectedIdentify string
		expectedType     PackageSource
	}{
		{
			name:             "oci",
			src:              "oci://ghcr.io/defenseunicorns/packages/init:1.0.0",
			expectedIdentify: "oci",
			expectedType:     &OCISource{},
		},
		{
			name:             "sget with sub path",
			src:              "sget://github.com/defenseunicorns/zarf-hello-world:x86",
			expectedIdentify: "sget",
			expectedType:     &URLSource{},
		},
		{
			name:             "sget without host",
			src:              "sget://defenseunicorns/zarf-hello-world:x86_64",
			expectedIdentify: "sget",
			expectedType:     &URLSource{},
		},
		{
			name:             "https",
			src:              "https://github.com/defenseunicorns/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst",
			expectedIdentify: "https",
			expectedType:     &URLSource{},
		},
		{
			name:             "http",
			src:              "http://github.com/defenseunicorns/zarf/releases/download/v1.0.0/zarf-init-amd64-v1.0.0.tar.zst",
			expectedIdentify: "http",
			expectedType:     &URLSource{},
		},
		{
			name:             "local tar init zst",
			src:              "zarf-init-amd64-v1.0.0.tar.zst",
			expectedIdentify: "tarball",
			expectedType:     &TarballSource{},
		},
		{
			name:             "local tar",
			src:              "zarf-package-manifests-amd64-v1.0.0.tar",
			expectedIdentify: "tarball",
			expectedType:     &TarballSource{},
		},
		{
			name:             "local tar manifest zst",
			src:              "zarf-package-manifests-amd64-v1.0.0.tar.zst",
			expectedIdentify: "tarball",
			expectedType:     &TarballSource{},
		},
		{
			name:             "local tar split",
			src:              "testdata/.part000",
			expectedIdentify: "split",
			expectedType:     &SplitTarballSource{},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expectedIdentify, Identify(tt.src))
			ps, err := New(&types.ZarfPackageOptions{PackageSource: tt.src})
			require.NoError(t, err)
			require.IsType(t, tt.expectedType, ps)
		})
	}
}

func TestPackageSource(t *testing.T) {
	t.Parallel()

	// Copy tar to a temp directory, otherwise Collect will delete it.
	tarName := "zarf-package-wordpress-amd64-16.0.4.tar.zst"
	testDir := t.TempDir()
	src, err := os.Open(filepath.Join("testdata", tarName))
	require.NoError(t, err)
	tarPath := filepath.Join(testDir, tarName)
	dst, err := os.Create(tarPath)
	require.NoError(t, err)
	_, err = io.Copy(dst, src)
	require.NoError(t, err)
	src.Close()
	dst.Close()

	b, err := os.ReadFile("./testdata/expected-pkg.json")
	require.NoError(t, err)
	expectedPkg := types.ZarfPackage{}
	err = json.Unmarshal(b, &expectedPkg)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, fp := filepath.Split(req.URL.Path)
		f, err := os.Open(filepath.Join("testdata", fp))
		if err != nil {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		defer f.Close()
		//nolint:errcheck // ignore
		io.Copy(rw, f)
	}))
	t.Cleanup(func() { ts.Close() })

	tests := []struct {
		name        string
		src         string
		shasum      string
		expectedErr string
	}{
		{
			name:        "local",
			src:         tarPath,
			expectedErr: "",
		},
		{
			name:        "http",
			src:         fmt.Sprintf("%s/zarf-package-wordpress-amd64-16.0.4.tar.zst", ts.URL),
			shasum:      "835b06fc509e639497fb45f45d432e5c4cbd5d84212db5357b16bc69724b0e26",
			expectedErr: "",
		},
		{
			name:        "http-insecure",
			src:         fmt.Sprintf("%s/zarf-package-wordpress-amd64-16.0.4.tar.zst", ts.URL),
			expectedErr: "remote package provided without a shasum, use --insecure to ignore, or provide one w/ --shasum",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// TODO once our messaging is thread safe, re-parallelize this test
			opts := &types.ZarfPackageOptions{
				PackageSource: tt.src,
				Shasum:        tt.shasum,
			}

			ps, err := New(opts)
			require.NoError(t, err)
			packageDir := t.TempDir()
			pkgLayout := layout.New(packageDir)
			pkg, warnings, err := ps.LoadPackage(context.Background(), pkgLayout, filters.Empty(), false)
			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Empty(t, warnings)
			require.Equal(t, expectedPkg, pkg)

			ps, err = New(opts)
			require.NoError(t, err)
			metadataDir := t.TempDir()
			metadataLayout := layout.New(metadataDir)
			metadata, warnings, err := ps.LoadPackageMetadata(context.Background(), metadataLayout, true, false)
			require.NoError(t, err)
			require.Empty(t, warnings)
			require.Equal(t, expectedPkg, metadata)

			ps, err = New(opts)
			require.NoError(t, err)
			collectDir := t.TempDir()
			fp, err := ps.Collect(context.Background(), collectDir)
			require.NoError(t, err)
			require.Equal(t, filepath.Join(collectDir, filepath.Base(tt.src)), fp)
		})
	}
}
