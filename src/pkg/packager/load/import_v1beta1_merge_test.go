// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestFixActionPathsV1Beta1(t *testing.T) {
	t.Parallel()

	customDir := "custom/path"
	actions := []v1beta1.ComponentAction{
		{
			Cmd: "echo explicit-dir",
			Dir: &customDir,
		},
		{
			Cmd: "echo default-dir",
			Dir: nil,
		},
	}

	fixed := fixActionPathsV1Beta1(actions, "default/dir", "components/sub")
	require.Len(t, fixed, 2)
	require.NotNil(t, fixed[0].Dir)
	require.Equal(t, "components/sub/custom/path", *fixed[0].Dir)
	require.NotNil(t, fixed[1].Dir)
	require.Equal(t, "components/sub/default/dir", *fixed[1].Dir)
}

func TestFixPathsV1Beta1(t *testing.T) {
	t.Parallel()

	actionDir := "action-scripts"
	spec := v1beta1.ComponentSpec{
		Files: []v1beta1.File{
			{Source: "files/test.txt"},
			{Source: "https://example.com/remote.txt"},
		},
		ImageArchives: []v1beta1.ImageArchive{
			{Path: "images/archive.tar"},
		},
		Charts: []v1beta1.Chart{
			{
				Name:  "my-chart",
				Local: &v1beta1.LocalSource{Path: "charts/local-chart"},
				ValuesFiles: []v1beta1.ValuesFile{
					{Path: "charts/values.yaml"},
				},
			},
		},
		Manifests: []v1beta1.Manifest{
			{
				Name:  "my-manifest",
				Files: []string{"manifests/deploy.yaml"},
				Kustomize: v1beta1.KustomizeManifest{
					Files: []string{"manifests/kustomization"},
				},
			},
		},
		Actions: v1beta1.ComponentActions{
			OnCreate: v1beta1.ComponentActionSet{
				Defaults: &v1beta1.ComponentActionDefaults{Dir: "defaults/dir"},
				Before: []v1beta1.ComponentAction{
					{Cmd: "echo before", Dir: &actionDir},
				},
				OnSuccess: []v1beta1.ComponentAction{
					{Cmd: "echo success", Dir: nil},
				},
				OnFailure: []v1beta1.ComponentAction{
					{Cmd: "echo failure", Dir: &actionDir},
				},
			},
		},
	}

	rebased := fixPathsV1Beta1(spec, "components/child")

	require.Equal(t, "components/child/files/test.txt", rebased.Files[0].Source)
	require.Equal(t, "https://example.com/remote.txt", rebased.Files[1].Source, "URLs must not be prepended with relativeToHead")
	require.Equal(t, "components/child/images/archive.tar", rebased.ImageArchives[0].Path)
	require.Equal(t, "components/child/charts/local-chart", rebased.Charts[0].Local.Path)
	require.Equal(t, "components/child/charts/values.yaml", rebased.Charts[0].ValuesFiles[0].Path)
	require.Equal(t, "components/child/manifests/deploy.yaml", rebased.Manifests[0].Files[0])
	require.Equal(t, "components/child/manifests/kustomization", rebased.Manifests[0].Kustomize.Files[0])

	require.Equal(t, "components/child/action-scripts", *rebased.Actions.OnCreate.Before[0].Dir)
	require.Equal(t, "components/child/defaults/dir", *rebased.Actions.OnCreate.OnSuccess[0].Dir)
	require.Equal(t, "components/child/action-scripts", *rebased.Actions.OnCreate.OnFailure[0].Dir)
}

func TestMergeManifests(t *testing.T) {
	t.Parallel()

	base := []v1beta1.Manifest{
		{
			Name:             "common-manifest",
			Namespace:        "base-ns",
			Files:            []string{"base.yaml"},
			SkipWait:         false,
			ServerSideApply:  v1beta1.ServerSideApplyAuto,
			EnableTemplating: false,
		},
		{
			Name:  "base-only",
			Files: []string{"base-only.yaml"},
		},
	}

	head := []v1beta1.Manifest{
		{
			Name:             "common-manifest",
			Namespace:        "override-ns",
			Files:            []string{"override.yaml"},
			SkipWait:         true,
			ServerSideApply:  v1beta1.ServerSideApplyEnabled,
			EnableTemplating: true,
			Kustomize: v1beta1.KustomizeManifest{
				Files:             []string{"kustomize-dir"},
				AllowAnyDirectory: true,
				EnablePlugins:     true,
			},
		},
		{
			Name:  "head-only",
			Files: []string{"head-only.yaml"},
		},
	}

	merged := mergeManifests(base, head)
	require.Len(t, merged, 3)

	common := merged[0]
	require.Equal(t, "common-manifest", common.Name)
	require.Equal(t, "override-ns", common.Namespace)
	require.Equal(t, []string{"base.yaml", "override.yaml"}, common.Files)
	require.True(t, common.SkipWait)
	require.Equal(t, v1beta1.ServerSideApplyEnabled, common.ServerSideApply)
	require.True(t, common.EnableTemplating)
	require.Equal(t, []string{"kustomize-dir"}, common.Kustomize.Files)
	require.True(t, common.Kustomize.AllowAnyDirectory)
	require.True(t, common.Kustomize.EnablePlugins)

	require.Equal(t, "base-only", merged[1].Name)
	require.Equal(t, "head-only", merged[2].Name)
}

func TestMergeCharts(t *testing.T) {
	t.Parallel()

	t.Run("merges charts and overrides fields", func(t *testing.T) {
		t.Parallel()

		base := []v1beta1.Chart{
			{
				Name:      "app-chart",
				Namespace: "default",
				HelmRepository: &v1beta1.HelmRepositorySource{
					URL:     "https://charts.example.com",
					Name:    "app-repo",
					Version: "1.0.0",
				},
				ValuesFiles: []v1beta1.ValuesFile{{Path: "base-values.yaml"}},
			},
		}

		head := []v1beta1.Chart{
			{
				Name:                 "app-chart",
				Namespace:            "custom-ns",
				ReleaseName:          "custom-release",
				SkipWait:             true,
				SkipSchemaValidation: true,
				ServerSideApply:      v1beta1.ServerSideApplyEnabled,
				HelmRepository: &v1beta1.HelmRepositorySource{
					Version: "2.0.0",
				},
				ValuesFiles: []v1beta1.ValuesFile{{Path: "head-values.yaml"}},
			},
		}

		merged, err := mergeCharts(base, head)
		require.NoError(t, err)
		require.Len(t, merged, 1)

		chart := merged[0]
		require.Equal(t, "custom-ns", chart.Namespace)
		require.Equal(t, "custom-release", chart.ReleaseName)
		require.True(t, chart.SkipWait)
		require.True(t, chart.SkipSchemaValidation)
		require.Equal(t, v1beta1.ServerSideApplyEnabled, chart.ServerSideApply)
		require.Equal(t, "2.0.0", chart.HelmRepository.Version)
		require.Equal(t, "https://charts.example.com", chart.HelmRepository.URL)
		require.Equal(t, []v1beta1.ValuesFile{
			{Path: "base-values.yaml"},
			{Path: "head-values.yaml"},
		}, chart.ValuesFiles)
	})

	t.Run("rejects invalid source override", func(t *testing.T) {
		t.Parallel()

		base := []v1beta1.Chart{
			{
				Name: "app-chart",
				HelmRepository: &v1beta1.HelmRepositorySource{
					URL:  "https://charts.example.com/one",
					Name: "repo",
				},
			},
		}

		head := []v1beta1.Chart{
			{
				Name: "app-chart",
				HelmRepository: &v1beta1.HelmRepositorySource{
					URL: "https://charts.example.com/different",
				},
			},
		}

		_, err := mergeCharts(base, head)
		require.ErrorContains(t, err, "cannot override helmRepository.url")
	})
}

func TestMergeImages(t *testing.T) {
	t.Parallel()

	base := []v1beta1.Image{
		{ID: "img-1", Name: "app:1.0", Source: v1beta1.ImageSourceRegistry},
		{ID: "redis", Name: "redis:6.0"},
		{Name: "nginx:1.26", Source: v1beta1.ImageSourceRegistry},
	}

	head := []v1beta1.Image{
		{ID: "img-1", Name: "app:2.0", Source: v1beta1.ImageSourceDaemon},
		{ID: "redis", Name: "redis:7.0"},
		{Name: "nginx:1.26", Source: v1beta1.ImageSourceDaemon},
		{Name: "sidecar:latest"},
	}

	merged := mergeImages(base, head)
	require.Len(t, merged, 4)

	require.Equal(t, "img-1", merged[0].ID)
	require.Equal(t, "app:2.0", merged[0].Name)
	require.Equal(t, v1beta1.ImageSourceDaemon, merged[0].Source)

	require.Equal(t, "redis", merged[1].ID)
	require.Equal(t, "redis:7.0", merged[1].Name)

	require.Equal(t, "nginx:1.26", merged[2].Name)
	require.Equal(t, v1beta1.ImageSourceDaemon, merged[2].Source)

	require.Equal(t, "sidecar:latest", merged[3].Name)
}
