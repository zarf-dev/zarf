// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestPackageGetAPIVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		apiVersion string
		want       string
	}{
		{
			name: "defaults omitted version to v1alpha1",
			want: v1alpha1.APIVersion,
		},
		{
			name:       "preserves specified version",
			apiVersion: v1beta1.APIVersion,
			want:       v1beta1.APIVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, api.Package{APIVersion: tt.apiVersion}.GetAPIVersion())
		})
	}
}

func TestPackageHasImages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pkg  api.Package
		want bool
	}{
		{
			name: "returns false without images",
			pkg:  api.Package{Components: []api.Component{{}}},
		},
		{
			name: "finds direct images",
			pkg:  api.Package{Components: []api.Component{{Images: []api.Image{{Name: "registry.example/app:1.0.0"}}}}},
			want: true,
		},
		{
			name: "finds image archives",
			pkg:  api.Package{Components: []api.Component{{ImageArchives: []api.ImageArchive{{Path: "images.tar"}}}}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.pkg.HasImages())
		})
	}
}

func TestComponentRequiresCluster(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		component api.Component
		want      bool
	}{
		{name: "does not require a cluster without cluster resources"},
		{name: "images require a cluster", component: api.Component{Images: []api.Image{{Name: "registry.example/app:1.0.0"}}}, want: true},
		{name: "image archives require a cluster", component: api.Component{ImageArchives: []api.ImageArchive{{Path: "images.tar"}}}, want: true},
		{name: "charts require a cluster", component: api.Component{Charts: []api.Chart{{Name: "chart"}}}, want: true},
		{name: "manifests require a cluster", component: api.Component{Manifests: []api.Manifest{{Name: "manifest"}}}, want: true},
		{name: "repositories require a cluster", component: api.Component{Repositories: []api.Repository{{URL: "https://example.com/repo.git"}}}, want: true},
		{name: "data injections require a cluster", component: api.Component{DataInjections: []api.ZarfDataInjection{{}}}, want: true},
		{name: "health checks require a cluster", component: api.Component{HealthChecks: []api.NamespacedObjectKindReference{{}}}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.component.RequiresCluster())
		})
	}
}

func TestComponentGetImages(t *testing.T) {
	t.Parallel()

	component := api.Component{
		Images: []api.Image{{Name: "registry.example/direct:1.0.0"}},
		ImageArchives: []api.ImageArchive{
			{Path: "first.tar", Images: []string{"registry.example/first:1.0.0"}},
			{Path: "second.tar", Images: []string{"registry.example/second:1.0.0", "registry.example/third:1.0.0"}},
		},
	}

	require.Equal(t, []string{
		"registry.example/direct:1.0.0",
		"registry.example/first:1.0.0",
		"registry.example/second:1.0.0",
		"registry.example/third:1.0.0",
	}, component.GetImages())
}

func TestChartAccessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		chart              api.Chart
		wantSourceURL      string
		wantLocalPath      string
		wantRepositoryName string
		wantGitPath        string
		wantApply          string
	}{
		{
			name:               "helm repository source",
			chart:              api.Chart{HelmRepository: &api.HelmRepositorySource{Name: "upstream", URL: "https://charts.example.com", Version: "1.0.0"}},
			wantSourceURL:      "https://charts.example.com",
			wantRepositoryName: "upstream",
			wantApply:          "auto",
		},
		{
			name:          "git source",
			chart:         api.Chart{Git: &api.GitSource{URL: "https://example.com/repo.git", Path: "charts/app"}},
			wantSourceURL: "https://example.com/repo.git",
			wantGitPath:   "charts/app",
			wantApply:     "auto",
		},
		{
			name:          "OCI source",
			chart:         api.Chart{OCI: &api.OCISource{URL: "oci://registry.example/charts/app"}, ServerSideApply: "true"},
			wantSourceURL: "oci://registry.example/charts/app",
			wantApply:     "true",
		},
		{
			name:          "local source",
			chart:         api.Chart{Local: &api.LocalSource{Path: "charts/app"}},
			wantLocalPath: "charts/app",
			wantApply:     "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.wantSourceURL, tt.chart.SourceURL())
			require.Equal(t, tt.wantLocalPath, tt.chart.LocalPath())
			require.Equal(t, tt.wantRepositoryName, tt.chart.RepositoryName())
			require.Equal(t, tt.wantGitPath, tt.chart.GitPath())
			require.Equal(t, tt.wantApply, tt.chart.GetServerSideApply())
		})
	}
}

func TestManifestGetServerSideApply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "defaults to auto", want: "auto"},
		{name: "returns configured strategy", value: "false", want: "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, api.Manifest{ServerSideApply: tt.value}.GetServerSideApply())
		})
	}
}
