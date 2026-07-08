// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/types"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
)

func TestChartTemplate(t *testing.T) {
	ctx := context.Background()
	chartPath := filepath.Join("testdata", "template", "simple-chart")
	chart := v1alpha1.ZarfChart{
		Name:      "simple-chart",
		Version:   "1.0.0",
		LocalPath: chartPath,
	}
	tmpdir := t.TempDir()
	err := PackageChart(ctx, chart, tmpdir, tmpdir, tmpdir, types.RemoteOptions{})
	require.NoError(t, err)
	kubeVersion := ""
	vc := template.GetZarfVariableConfig(ctx, false)
	vc.SetVariable("image", "nginx:1.0.0", false, false, v1alpha1.RawVariableType)
	vc.SetVariable("port", "8080", false, false, v1alpha1.RawVariableType)
	helmChart, values, err := LoadChartData(chart, tmpdir, tmpdir, nil)
	require.NoError(t, err)
	manifest, err := TemplateChart(ctx, chart, helmChart, values, kubeVersion, vc, false, types.RemoteOptions{})
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join("testdata", "template", "expected", "manifest.yaml"))
	require.NoError(t, err)
	require.YAMLEq(t, string(b), manifest)
}

func TestChartTemplate_DoesNotNegotiateDeclaredOCIDependencies(t *testing.T) {
	// TemplateChart operates on an already-loaded chart; Helm's RunWithContext never
	// consults client.PlainHTTP for one (see the comment in TemplateChart). Declaring
	// an OCI dependency pointing at an unreachable host must not make TemplateChart
	// probe it, hang, or fail.
	ctx := context.Background()
	chartPath := filepath.Join("testdata", "template", "simple-chart")
	chart := v1alpha1.ZarfChart{
		Name:      "simple-chart",
		Version:   "1.0.0",
		LocalPath: chartPath,
	}
	tmpdir := t.TempDir()
	err := PackageChart(ctx, chart, tmpdir, tmpdir, tmpdir, types.RemoteOptions{})
	require.NoError(t, err)
	kubeVersion := ""
	vc := template.GetZarfVariableConfig(ctx, false)
	vc.SetVariable("image", "nginx:1.0.0", false, false, v1alpha1.RawVariableType)
	vc.SetVariable("port", "8080", false, false, v1alpha1.RawVariableType)
	helmChart, values, err := LoadChartData(chart, tmpdir, tmpdir, nil)
	require.NoError(t, err)

	// Nothing is listening on this port: a real probe would fail fast, but any
	// probe at all (fast or slow) means the dead negotiation regressed.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	helmChart.Metadata.Dependencies = []*chartv2.Dependency{
		{Name: "sub", Version: "1.0.0", Repository: "oci://" + addr + "/sub"},
	}

	start := time.Now()
	_, err = TemplateChart(ctx, chart, helmChart, values, kubeVersion, vc, false, types.RemoteOptions{PlainHTTP: true})
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.Less(t, elapsed, 2*time.Second, "TemplateChart must not negotiate declared OCI dependencies")
}
