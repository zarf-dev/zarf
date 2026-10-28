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
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/types"
)

func TestChartTemplate(t *testing.T) {
	ctx := context.Background()
	chartPath := filepath.Join("testdata", "template", "simple-chart")
	chart := api.Chart{
		Name:          "simple-chart",
		LegacyVersion: "1.0.0",
		Local:         &api.LocalSource{Path: chartPath},
	}
	tmpdir := t.TempDir()
	paths := layout.ChartPaths{ChartsDir: tmpdir, ValuesDir: tmpdir}
	err := PackageChart(ctx, chart, paths, tmpdir, types.RemoteOptions{})
	require.NoError(t, err)
	kubeVersion := ""
	vc := template.GetZarfVariableConfig(ctx, false)
	vc.SetVariable("image", "nginx:1.0.0", false, false, api.RawVariableType)
	vc.SetVariable("port", "8080", false, false, api.RawVariableType)
	helmChart, values, err := LoadChartData(chart, paths, nil)
	require.NoError(t, err)
	manifest, err := TemplateChart(ctx, chart, helmChart, values, kubeVersion, vc, false, types.RemoteOptions{})
	require.NoError(t, err)
	b, err := os.ReadFile(filepath.Join("testdata", "template", "expected", "manifest.yaml"))
	require.NoError(t, err)
	require.YAMLEq(t, string(b), manifest)
}
