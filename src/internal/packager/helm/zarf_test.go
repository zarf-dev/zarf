// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helm

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/packager/postrender"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func initPackageWithPostRenderers(renderer api.KustomizePostRenderer) api.Package {
	return api.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       api.ZarfInitConfig,
		Metadata:   api.PackageMetadata{Name: "init"},
		Components: []api.Component{
			{
				Name:    "zarf-registry",
				Service: api.Service{Name: api.ServiceRegistry},
				Charts: []api.Chart{
					{Name: "unrelated", Namespace: "zarf", PostRenderers: []api.PostRenderer{{Kustomize: &api.KustomizePostRenderer{}}}},
					{
						Name:               "docker-registry",
						ReleaseName:        "zarf-docker-registry",
						Namespace:          "zarf",
						PostRenderStrategy: api.PostRenderStrategySeparate,
						PostRenderers:      []api.PostRenderer{{Kustomize: &renderer}},
					},
				},
			},
		},
	}
}

const registryDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: zarf-docker-registry
spec:
  template:
    spec:
      containers:
        - name: registry
          image: registry:1
`

// renderedImage runs r over a registry Deployment and returns the resulting manifests.
func renderedImage(t *testing.T, r *postrender.Renderer) string {
	t.Helper()
	require.NotNil(t, r)
	out, err := r.Run(bytes.NewBufferString(registryDeployment))
	require.NoError(t, err)
	return out.String()
}

func TestDeployedPostRenderer(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	templated := api.KustomizePostRenderer{
		EnableTemplating: true,
		Images:           []api.KustomizeImage{{Name: "registry", NewTag: "{{ .Values.tag }}"}},
	}
	plain := api.KustomizePostRenderer{Images: []api.KustomizeImage{{Name: "registry", NewTag: "3"}}}
	registryChart := api.Chart{Namespace: "zarf", ReleaseName: "zarf-docker-registry"}
	opts := InstallUpgradeOptions{VariableConfig: template.GetZarfVariableConfig(ctx, false), State: &state.State{}}

	t.Run("templated post-renderers use the recorded deploy values", func(t *testing.T) {
		t.Parallel()
		deployed, err := state.NewDeployedPackage(initPackageWithPostRenderers(templated), "", "", nil, 1)
		require.NoError(t, err)
		deployed.SetDeployConfig(state.DeployConfig{Values: value.Values{"tag": "2.8.3"}})

		gotDeployed, gotPkg, err := findPackageWithService([]state.DeployedPackage{*deployed}, api.ServiceRegistry)
		require.NoError(t, err)

		r, err := deployedPostRenderer(ctx, gotDeployed, *gotPkg, api.ServiceRegistry, registryChart, opts)
		require.NoError(t, err)
		require.Contains(t, renderedImage(t, r), "image: registry:2.8.3")
	})

	t.Run("templates using unrecorded package values explain why they fail", func(t *testing.T) {
		t.Parallel()
		pkg := initPackageWithPostRenderers(templated)
		deployed, err := state.NewDeployedPackage(pkg, "", "", nil, 1)
		require.NoError(t, err)
		_, err = deployedPostRenderer(ctx, deployed, pkg, api.ServiceRegistry, registryChart, opts)
		require.ErrorContains(t, err, `chart "docker-registry": post-renderer at index 0: image override at index 0: templating failed`)
		require.ErrorContains(t, err, "package default values are not recorded")
	})

	t.Run("plain post-renderers need no template objects", func(t *testing.T) {
		t.Parallel()
		pkg := initPackageWithPostRenderers(plain)
		deployed, err := state.NewDeployedPackage(pkg, "", "", nil, 1)
		require.NoError(t, err)
		r, err := deployedPostRenderer(ctx, deployed, pkg, api.ServiceRegistry, registryChart, InstallUpgradeOptions{})
		require.NoError(t, err)
		require.Contains(t, renderedImage(t, r), "image: registry:3")

		// The release name defaults to the chart name on both sides of the match.
		r, err = deployedPostRenderer(ctx, deployed, pkg, api.ServiceRegistry, api.Chart{Name: "zarf-docker-registry", Namespace: "zarf"}, InstallUpgradeOptions{})
		require.NoError(t, err)
		require.Contains(t, renderedImage(t, r), "image: registry:3")
	})

	t.Run("other services and releases have no post-renderer", func(t *testing.T) {
		t.Parallel()
		pkg := initPackageWithPostRenderers(plain)
		deployed, err := state.NewDeployedPackage(pkg, "", "", nil, 1)
		require.NoError(t, err)
		for _, tc := range []struct {
			chart   api.Chart
			service api.ServiceName
		}{
			{chart: registryChart, service: api.ServiceAgent},
			{chart: api.Chart{Namespace: "zarf", ReleaseName: "zarf-agent"}, service: api.ServiceRegistry},
			{chart: api.Chart{Namespace: "other", ReleaseName: "zarf-docker-registry"}, service: api.ServiceRegistry},
		} {
			r, err := deployedPostRenderer(ctx, deployed, pkg, tc.service, tc.chart, InstallUpgradeOptions{})
			require.NoError(t, err)
			require.Nil(t, r)
		}
	})
}
