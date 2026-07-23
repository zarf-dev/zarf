// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/types"
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

func TestResolveCrossDocumentAnchors(t *testing.T) {
	t.Parallel()

	// Mirrors the corrupted stream Helm v4's annotateAndMerge produces from a
	// `kind: List` with a cross-item anchor: the anchor lands in document 1 and
	// the alias in document 2, leaving the alias dangling (zarf #4977).
	crossDocAnchor := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-a
data: &shared
  key: value
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-b
data: *shared
`

	tests := []struct {
		name        string
		content     string
		wantRepair  bool // true if the helper should return repaired bytes
		wantContent map[string]string
	}{
		{
			name:        "cross-document mapping alias is materialized",
			content:     crossDocAnchor,
			wantRepair:  true,
			wantContent: map[string]string{"key: value": ""},
		},
		{
			name: "cross-document sequence alias is materialized",
			content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-a
data:
  owner: &val zarf
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-b
data:
  owners:
  - *val
`,
			wantRepair:  true,
			wantContent: map[string]string{"- zarf": ""},
		},
		{
			name: "no anchors is a no-op",
			content: `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-a
data:
  key: value
`,
			wantRepair: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The dangling alias must break the standard sorter, proving the repair is needed.
			if tt.wantRepair {
				_, _, err := releaseutil.SortManifests(map[string]string{"manifest": tt.content}, nil, releaseutil.InstallOrder)
				require.Error(t, err, "expected unrepaired content to fail SortManifests")
			}

			repaired, err := resolveCrossDocumentAnchors([]byte(tt.content))
			require.NoError(t, err)

			if !tt.wantRepair {
				require.Nil(t, repaired, "no-op input should return nil")
				return
			}

			require.NotNil(t, repaired)
			// The alias marker is gone and the repaired stream parses cleanly.
			require.NotContains(t, string(repaired), "*")
			_, resources, err := releaseutil.SortManifests(map[string]string{"manifest": string(repaired)}, nil, releaseutil.InstallOrder)
			require.NoError(t, err, "repaired content should parse: %s", string(repaired))
			require.Len(t, resources, 2)

			for want := range tt.wantContent {
				require.True(t, strings.Contains(resources[0].Content, want) || strings.Contains(resources[1].Content, want),
					"materialized value %q not found in resources", want)
			}
		})
	}
}
