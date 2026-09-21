// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/registry"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

func TestPackageChartValidatesSource(t *testing.T) {
	t.Parallel()

	validGit := &api.GitSource{
		URL: "https://github.com/example/chart.git",
		Ref: &api.GitRef{Tag: "v1.2.3"},
	}
	tests := []struct {
		name  string
		chart api.Chart
		want  string
	}{
		{
			name:  "no source",
			chart: api.Chart{Name: "chart"},
			want:  "must specify exactly one source",
		},
		{
			name: "multiple sources",
			chart: api.Chart{
				Name:  "chart",
				Git:   validGit,
				Local: &api.LocalSource{Path: "charts/local"},
			},
			want: "must specify exactly one source",
		},
		{
			name:  "git without URL",
			chart: api.Chart{Name: "chart", Git: &api.GitSource{Ref: &api.GitRef{Tag: "v1.2.3"}}},
			want:  "must specify a URL",
		},
		{
			name:  "git without ref",
			chart: api.Chart{Name: "chart", Git: &api.GitSource{URL: validGit.URL}},
			want:  "must specify exactly one ref",
		},
		{
			name: "git with multiple refs",
			chart: api.Chart{
				Name: "chart",
				Git:  &api.GitSource{URL: validGit.URL, Ref: &api.GitRef{Tag: "v1.2.3", Branch: "main"}},
			},
			want: "must specify exactly one ref",
		},
		{
			name: "git with invalid commit",
			chart: api.Chart{
				Name: "chart",
				Git:  &api.GitSource{URL: validGit.URL, Ref: &api.GitRef{Commit: "not-a-commit"}},
			},
			want: "commit must be a 40-character SHA-1",
		},
		{
			name:  "local without path",
			chart: api.Chart{Name: "chart", Local: &api.LocalSource{}},
			want:  "must specify a path",
		},
		{
			name:  "helm repository without URL",
			chart: api.Chart{Name: "chart", HelmRepository: &api.HelmRepositorySource{Version: "1.2.3"}},
			want:  "must specify a URL",
		},
		{
			name:  "helm repository without version",
			chart: api.Chart{Name: "chart", HelmRepository: &api.HelmRepositorySource{URL: "https://charts.example.com"}},
			want:  "must specify a version",
		},
		{
			name:  "OCI without URL",
			chart: api.Chart{Name: "chart", OCI: &api.OCISource{Ref: &api.OCIRef{Tag: "1.2.3"}}},
			want:  "must specify a URL",
		},
		{
			name:  "OCI without ref",
			chart: api.Chart{Name: "chart", OCI: &api.OCISource{URL: "oci://registry.example.com/chart"}},
			want:  "must specify exactly one ref",
		},
		{
			name: "OCI with multiple refs",
			chart: api.Chart{
				Name: "chart",
				OCI:  &api.OCISource{URL: "oci://registry.example.com/chart", Ref: &api.OCIRef{Tag: "1.2.3", Digest: "sha256:abc"}},
			},
			want: "must specify exactly one ref",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := PackageChart(testutil.TestContext(t), tt.chart, layout.ChartPaths{
				ChartsDir: t.TempDir(),
				ValuesDir: t.TempDir(),
			}, t.TempDir(), types.RemoteOptions{})
			require.ErrorContains(t, err, tt.want)
		})
	}
}

// TestDownloadPublishedChartResolvesToOCI covers a classic Helm repository whose
// index redirects a chart to an OCI reference (as Bitnami's does). Helm's
// downloader requires a registry client to resolve an OCI ref, so Zarf must
// provision one even though the chart URL is a plain HTTP repository.
func TestDownloadPublishedChartResolvesToOCI(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	// Package a minimal chart into a tarball to push to the registry.
	ch := &chartv2.Chart{Metadata: &chartv2.Metadata{
		APIVersion: chartv2.APIVersionV1,
		Name:       "simple-chart",
		Version:    "1.0.0",
	}}
	tgzPath, err := chartutil.Save(ch, t.TempDir())
	require.NoError(t, err)
	chartData, err := os.ReadFile(tgzPath)
	require.NoError(t, err)

	// Push the chart to an in-memory OCI registry over plain HTTP.
	regAddr := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	regClient, err := registry.NewClient(registry.ClientOptPlainHTTP())
	require.NoError(t, err)
	ociRef := fmt.Sprintf("%s/charts/simple-chart:1.0.0", regAddr)
	_, err = regClient.Push(chartData, ociRef)
	require.NoError(t, err)

	// Serve a classic Helm repo index that points the chart at the OCI ref.
	index := fmt.Sprintf(`apiVersion: v1
entries:
  simple-chart:
  - apiVersion: v2
    name: simple-chart
    version: 1.0.0
    urls:
    - oci://%s
`, ociRef)
	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(index)) //nolint:errcheck
	})
	repoSrv := httptest.NewServer(mux)
	defer repoSrv.Close()

	chart := api.Chart{
		Name:           "simple-chart",
		LegacyVersion:  "archive-version",
		HelmRepository: &api.HelmRepositorySource{URL: repoSrv.URL, Version: "1.0.0"},
	}
	chartPath := t.TempDir()
	paths := layout.ChartPaths{ChartsDir: chartPath, ValuesDir: t.TempDir()}
	err = PackageChart(ctx, chart, paths, t.TempDir(), types.RemoteOptions{
		PlainHTTP:             true,
		InsecureSkipTLSVerify: true,
	})
	require.NoError(t, err)
	require.FileExists(t, paths.Archive(chart.Name, chart.LegacyVersion))
}

func TestDownloadPublishedChartFromOCI(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	// Package a minimal chart into a tarball to push to the registry.
	ch := &chartv2.Chart{Metadata: &chartv2.Metadata{
		APIVersion: chartv2.APIVersionV1,
		Name:       "simple-chart",
		Version:    "1.0.0",
	}}
	tgzPath, err := chartutil.Save(ch, t.TempDir())
	require.NoError(t, err)
	chartData, err := os.ReadFile(tgzPath)
	require.NoError(t, err)

	// Push the chart to an in-memory OCI registry over plain HTTP.
	regAddr := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	regClient, err := registry.NewClient(registry.ClientOptPlainHTTP())
	require.NoError(t, err)
	_, err = regClient.Push(chartData, fmt.Sprintf("%s/charts/simple-chart:1.0.0", regAddr))
	require.NoError(t, err)

	chart := api.Chart{
		Name:          "simple-chart",
		LegacyVersion: "archive-version",
		OCI: &api.OCISource{
			URL: fmt.Sprintf("oci://%s/charts/simple-chart", regAddr),
			Ref: &api.OCIRef{Tag: "1.0.0"},
		},
	}
	chartPath := t.TempDir()
	paths := layout.ChartPaths{ChartsDir: chartPath, ValuesDir: t.TempDir()}
	err = PackageChart(ctx, chart, paths, t.TempDir(), types.RemoteOptions{
		PlainHTTP:             true,
		InsecureSkipTLSVerify: true,
	})
	require.NoError(t, err)
	require.FileExists(t, paths.Archive(chart.Name, chart.LegacyVersion))
}
