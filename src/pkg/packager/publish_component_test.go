// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestComponentResourcesRejectUnsupportedRemoteSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component v1beta1.ComponentConfig
		wantErr   string
	}{
		{
			name: "values file",
			component: v1beta1.ComponentConfig{
				Values: v1beta1.Values{Files: []string{"https://example.com/values.yaml"}},
			},
			wantErr: "remote values files are not supported",
		},
		{
			name: "values schema",
			component: v1beta1.ComponentConfig{
				Values: v1beta1.Values{Schema: "https://example.com/values.schema.json"},
			},
			wantErr: "remote values schemas are not supported",
		},
		{
			name: "local chart",
			component: v1beta1.ComponentConfig{
				Component: v1beta1.ComponentSpec{Charts: []v1beta1.Chart{{Local: &v1beta1.LocalSource{Path: "https://example.com/chart.tgz"}}}},
			},
			wantErr: "remote local chart paths are not supported",
		},
		{
			name: "image archive",
			component: v1beta1.ComponentConfig{
				Component: v1beta1.ComponentSpec{ImageArchives: []v1beta1.ImageArchive{{Path: "https://example.com/images.tar"}}},
			},
			wantErr: "remote image archive paths are not supported",
		},
		{
			name: "component import",
			component: v1beta1.ComponentConfig{
				Component: v1beta1.ComponentSpec{Import: v1beta1.ComponentImport{Remote: []v1beta1.ComponentImportRemote{{URL: "oci://example.com/components/foo:1.0.0"}}}},
			},
			wantErr: "remote component imports are not yet supported for v1beta1 packages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := componentResources(filepath.Join(t.TempDir(), "component.yaml"), tt.component, t.TempDir(), map[string]bool{})
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestComponentResourcesAllowsSupportedRemoteSources(t *testing.T) {
	t.Parallel()

	component := v1beta1.ComponentConfig{
		Component: v1beta1.ComponentSpec{
			Charts: []v1beta1.Chart{{ValuesFiles: []v1beta1.ValuesFile{{Path: "https://example.com/chart-values.yaml"}}}},
			Manifests: []v1beta1.Manifest{{
				Files:     []string{"https://example.com/manifest.yaml"},
				Kustomize: &v1beta1.KustomizeManifest{Files: []string{"https://example.com/kustomization.yaml"}},
			}},
			Files: []v1beta1.File{{Source: "https://example.com/file.txt"}},
		},
	}
	resources, err := componentResources(filepath.Join(t.TempDir(), "component.yaml"), component, t.TempDir(), map[string]bool{})
	require.NoError(t, err)
	require.Empty(t, resources)
}
