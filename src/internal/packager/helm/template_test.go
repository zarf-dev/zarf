// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
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
	err := PackageChart(ctx, chart, tmpdir, tmpdir)
	require.NoError(t, err)
	kubeVersion := ""
	vc := template.GetZarfVariableConfig(ctx)
	vc.SetVariable("image", "nginx:1.0.0", false, false, v1alpha1.RawVariableType)
	vc.SetVariable("port", "8080", false, false, v1alpha1.RawVariableType)
	helmChart, values, err := LoadChartData(chart, tmpdir, tmpdir, nil)
	require.NoError(t, err)
	manifest, err := TemplateChart(ctx, chart, helmChart, values, kubeVersion, vc)
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join("testdata", "template", "expected", "manifest.yaml"))
	require.NoError(t, err)
	require.YAMLEq(t, string(b), manifest)
}
