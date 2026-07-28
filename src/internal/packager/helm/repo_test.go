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

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"github.com/zarf-dev/zarf/src/types"
)

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

	chart := v1alpha1.ZarfChart{
		Name:    "simple-chart",
		Version: "1.0.0",
		URL:     repoSrv.URL,
	}
	chartPath := t.TempDir()
	err = PackageChart(ctx, chart, chartPath, t.TempDir(), t.TempDir(), types.RemoteOptions{
		PlainHTTP:             true,
		InsecureSkipTLSVerify: true,
	})
	require.NoError(t, err)
	require.FileExists(t, StandardName(chartPath, chart)+".tgz")
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

	chart := v1alpha1.ZarfChart{
		Name:    "simple-chart",
		Version: "1.0.0",
		URL:     fmt.Sprintf("oci://%s/charts/simple-chart", regAddr),
	}
	chartPath := t.TempDir()
	err = PackageChart(ctx, chart, chartPath, t.TempDir(), t.TempDir(), types.RemoteOptions{
		PlainHTTP:             true,
		InsecureSkipTLSVerify: true,
	})
	require.NoError(t, err)
	require.FileExists(t, StandardName(chartPath, chart)+".tgz")
}
