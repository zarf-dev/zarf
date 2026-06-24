// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package convert

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
)

func TestV1Alpha1PkgToV1Beta1_Metadata(t *testing.T) {
	t.Parallel()
	allowOverride := true
	pkg := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name:                   "test-pkg",
			Description:            "A test package",
			Version:                "1.0.0",
			URL:                    "https://example.com",
			Image:                  "https://example.com/image.png",
			Authors:                "Test Author",
			Documentation:          "https://docs.example.com",
			Source:                 "https://github.com/example",
			Vendor:                 "Example Corp",
			AggregateChecksum:      "abc123",
			Architecture:           "amd64",
			Uncompressed:           true,
			AllowNamespaceOverride: &allowOverride,
			Annotations: map[string]string{
				"existing": "annotation",
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Equal(t, v1beta1.APIVersion, result.APIVersion)
	require.Equal(t, v1beta1.ZarfPackageConfig, result.Kind)
	require.Equal(t, "test-pkg", result.Metadata.Name)
	require.Equal(t, "A test package", result.Metadata.Description)
	require.Equal(t, "1.0.0", result.Metadata.Version)
	require.Equal(t, "amd64", result.Metadata.Architecture)
	require.True(t, result.Metadata.Uncompressed)
	// AllowNamespaceOverride=true → PreventNamespaceOverride=false.
	require.False(t, result.Metadata.PreventNamespaceOverride)

	// v1alpha1-only metadata fields should be migrated to annotations.
	require.Equal(t, "https://example.com", result.Metadata.Annotations["metadata.url"])
	require.Equal(t, "https://example.com/image.png", result.Metadata.Annotations["metadata.image"])
	require.Equal(t, "Test Author", result.Metadata.Annotations["metadata.authors"])
	require.Equal(t, "https://docs.example.com", result.Metadata.Annotations["metadata.documentation"])
	require.Equal(t, "https://github.com/example", result.Metadata.Annotations["metadata.source"])
	require.Equal(t, "Example Corp", result.Metadata.Annotations["metadata.vendor"])
	// Existing annotation should be preserved.
	require.Equal(t, "annotation", result.Metadata.Annotations["existing"])

	// AggregateChecksum should move from metadata to build.
	require.Equal(t, "abc123", result.Build.AggregateChecksum)
}

func TestV1Alpha1PkgToV1Beta1_Build(t *testing.T) {
	t.Parallel()
	signed := true
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Build: v1alpha1.ZarfBuildData{
			Terminal:                   "my-machine",
			User:                       "test-user",
			Architecture:               "arm64",
			Timestamp:                  "Mon, 02 Jan 2006 15:04:05 -0700",
			Version:                    "v0.30.0",
			Migrations:                 []string{"migration1"},
			RegistryOverrides:          map[string]string{"docker.io": "internal.registry"},
			Differential:               true,
			DifferentialPackageVersion: "0.29.0",
			DifferentialMissing:        []string{"comp-a"},
			Flavor:                     "vanilla",
			Signed:                     &signed,
			VersionRequirements: []v1alpha1.VersionRequirement{
				{Version: "v0.28.0", Reason: "needs feature X"},
			},
			ProvenanceFiles: []string{"sig.json"},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Equal(t, v1beta1.ZarfPackageConfig, result.Kind)
	require.Equal(t, "my-machine", result.Build.Hostname)
	require.Equal(t, "test-user", result.Build.User)
	require.Equal(t, "arm64", result.Build.Architecture)
	require.Equal(t, "v0.30.0", result.Build.Version)
	require.True(t, result.Build.Differential)
	require.Equal(t, "0.29.0", result.Build.DifferentialPackageVersion)
	require.Equal(t, "vanilla", result.Build.Flavor)
	require.NotNil(t, result.Build.Signed)
	require.True(t, *result.Build.Signed)
	require.Len(t, result.Build.VersionRequirements, 1)
	require.Equal(t, "v0.28.0", result.Build.VersionRequirements[0].Version)
	require.Equal(t, "needs feature X", result.Build.VersionRequirements[0].Reason)
	require.Equal(t, []string{"sig.json"}, result.Build.ProvenanceFiles)
}

func TestV1Alpha1PkgToV1Beta1_VariablesAndConstantsShim(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Variables: []v1alpha1.InteractiveVariable{
			{
				Variable: v1alpha1.Variable{
					Name:       "MY_VAR",
					Sensitive:  true,
					AutoIndent: true,
					Pattern:    "^[a-z]+$",
					Type:       v1alpha1.FileVariableType,
				},
				Description: "A variable",
				Default:     "default-val",
				Prompt:      true,
			},
		},
		Constants: []v1alpha1.Constant{
			{
				Name:        "MY_CONST",
				Value:       "const-val",
				Description: "A constant",
				AutoIndent:  true,
				Pattern:     ".*",
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	vars := result.GetDeprecatedVariables()
	require.Len(t, vars, 1)
	require.Equal(t, "MY_VAR", vars[0].Name)
	require.True(t, vars[0].Sensitive)
	require.True(t, vars[0].AutoIndent)
	require.Equal(t, "^[a-z]+$", vars[0].Pattern)
	require.Equal(t, v1beta1.FileVariableType, vars[0].Type)
	require.Equal(t, "A variable", vars[0].Description)
	require.Equal(t, "default-val", vars[0].Default)
	require.True(t, vars[0].Prompt)

	consts := result.GetDeprecatedConstants()
	require.Len(t, consts, 1)
	require.Equal(t, "MY_CONST", consts[0].Name)
	require.Equal(t, "const-val", consts[0].Value)
	require.Equal(t, "A constant", consts[0].Description)
	require.True(t, consts[0].AutoIndent)
}

func TestV1Alpha1PkgToV1Beta1_YOLOAndGroupShim(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name: "yolo-pkg",
			YOLO: true,
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name:            "comp",
				DeprecatedGroup: "my-group",
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.True(t, result.Metadata.GetDeprecatedYOLO())
	require.Len(t, result.Components, 1)
	require.Equal(t, "my-group", result.Components[0].GetDeprecatedGroup())
}

func TestV1Alpha1PkgToV1Beta1_ComponentBasics(t *testing.T) {
	t.Parallel()
	required := true
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Components: []v1alpha1.ZarfComponent{
			{
				Name:        "my-component",
				Description: "test component",
				Required:    &required,
				Only: v1alpha1.ZarfComponentOnlyTarget{
					LocalOS: "linux",
					Cluster: v1alpha1.ZarfComponentOnlyCluster{
						Architecture: "amd64",
					},
					Flavor: "vanilla",
				},
				Import: v1alpha1.ZarfComponentImport{
					Path: "./path",
					URL:  "oci://example.com/pkg",
				},
				Images: []string{"nginx:latest", "redis:7"},
				Repos:  []string{"https://github.com/example/repo", "https://github.com/example/other"},
				StateAccess: []v1alpha1.StateAccessKey{
					v1alpha1.StateAccessRegistryCredentials,
					v1alpha1.StateAccessGitCredentials,
				},
				DataInjections: []v1alpha1.ZarfDataInjection{
					{Source: "/data", Target: v1alpha1.ZarfContainerTarget{Namespace: "default", Selector: "app=test", Container: "main", Path: "/inject"}},
				},
				HealthChecks: []v1alpha1.NamespacedObjectKindReference{
					{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "my-deploy"},
					{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "my-pod"},
				},
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Len(t, result.Components, 1)
	comp := result.Components[0]
	require.Equal(t, "my-component", comp.Name)
	require.Equal(t, "test component", comp.Description)

	// Required=true → Optional=false.
	require.False(t, comp.Optional)

	require.Equal(t, "linux", comp.Target.OS)
	require.Equal(t, "amd64", comp.Target.Architecture)
	require.Equal(t, "vanilla", comp.Target.Flavor)

	// v1alpha1 Import.Path/URL get promoted into the v1beta1 Local/Remote lists.
	require.Len(t, comp.Import.Local, 1)
	require.Equal(t, "./path", comp.Import.Local[0].Path)
	require.Len(t, comp.Import.Remote, 1)
	require.Equal(t, "oci://example.com/pkg", comp.Import.Remote[0].URL)

	// Images are converted from []string to []Image.
	require.Len(t, comp.Images, 2)
	require.Equal(t, "nginx:latest", comp.Images[0].Name)
	require.Equal(t, "redis:7", comp.Images[1].Name)

	// v1alpha1 Repos ([]string) become v1beta1 Repository objects keyed by URL.
	require.Equal(t, []v1beta1.Repository{
		{URL: "https://github.com/example/repo"},
		{URL: "https://github.com/example/other"},
	}, comp.Repositories)

	// StateAccess keys carry over unchanged.
	require.Equal(t, []v1beta1.StateAccessKey{
		v1beta1.StateAccessRegistryCredentials,
		v1beta1.StateAccessGitCredentials,
	}, comp.StateAccess)

	// DataInjections should be preserved via the private shim.
	di := comp.GetDeprecatedDataInjections()
	require.Len(t, di, 1)
	require.Equal(t, "/data", di[0].Source)

	// HealthChecks should become onDeploy.onSuccess wait actions with kind in <kind>.<version>.<group> format.
	require.Len(t, comp.Actions.OnDeploy.OnSuccess, 2)
	require.NotNil(t, comp.Actions.OnDeploy.OnSuccess[0].Wait)
	require.NotNil(t, comp.Actions.OnDeploy.OnSuccess[0].Wait.Cluster)
	require.Equal(t, "Deployment.v1.apps", comp.Actions.OnDeploy.OnSuccess[0].Wait.Cluster.Kind)
	require.Equal(t, "my-deploy", comp.Actions.OnDeploy.OnSuccess[0].Wait.Cluster.Name)
	require.Equal(t, "default", comp.Actions.OnDeploy.OnSuccess[0].Wait.Cluster.Namespace)

	// Core API resources (no group) keep kind as-is.
	require.NotNil(t, comp.Actions.OnDeploy.OnSuccess[1].Wait)
	require.NotNil(t, comp.Actions.OnDeploy.OnSuccess[1].Wait.Cluster)
	require.Equal(t, "Pod", comp.Actions.OnDeploy.OnSuccess[1].Wait.Cluster.Kind)
	require.Equal(t, "my-pod", comp.Actions.OnDeploy.OnSuccess[1].Wait.Cluster.Name)
	require.Equal(t, "default", comp.Actions.OnDeploy.OnSuccess[1].Wait.Cluster.Namespace)
}

func TestV1Alpha1PkgToV1Beta1_ServiceInference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		compName string
		service  v1beta1.Service
	}{
		{name: "registry", compName: "zarf-registry", service: v1beta1.ServiceRegistry},
		{name: "seed registry", compName: "zarf-seed-registry", service: v1beta1.ServiceSeedRegistry},
		{name: "injector", compName: "zarf-injector", service: v1beta1.ServiceInjector},
		{name: "agent", compName: "zarf-agent", service: v1beta1.ServiceAgent},
		{name: "git server", compName: "git-server", service: v1beta1.ServiceGitServer},
		{name: "no service", compName: "my-app", service: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pkg := v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{Name: tt.compName},
				},
			}
			result := V1Alpha1PkgToV1Beta1(pkg)
			require.Len(t, result.Components, 1)
			require.Equal(t, tt.service, result.Components[0].Service)
		})
	}
}

func TestV1Alpha1PkgToV1Beta1_ChartSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		chart    v1alpha1.ZarfChart
		validate func(t *testing.T, c v1beta1.Chart)
	}{
		{
			name: "helm repo",
			chart: v1alpha1.ZarfChart{
				Name:     "podinfo",
				URL:      "https://stefanprodan.github.io/podinfo",
				RepoName: "podinfo",
				Version:  "6.4.0",
			},
			validate: func(t *testing.T, c v1beta1.Chart) {
				require.NotNil(t, c.HelmRepository)
				require.Equal(t, "https://stefanprodan.github.io/podinfo", c.HelmRepository.URL)
				require.Equal(t, "podinfo", c.HelmRepository.Name)
				require.Equal(t, "6.4.0", c.HelmRepository.Version)
			},
		},
		{
			name: "oci registry",
			chart: v1alpha1.ZarfChart{
				Name:    "podinfo",
				URL:     "oci://ghcr.io/stefanprodan/charts/podinfo",
				Version: "6.4.0",
			},
			validate: func(t *testing.T, c v1beta1.Chart) {
				require.NotNil(t, c.OCI)
				require.Equal(t, "oci://ghcr.io/stefanprodan/charts/podinfo", c.OCI.URL)
				require.Equal(t, "6.4.0", c.OCI.Version)
			},
		},
		{
			name: "git repo with version",
			chart: v1alpha1.ZarfChart{
				Name:    "my-chart",
				URL:     "https://github.com/example/repo",
				GitPath: "charts/my-chart",
				Version: "6.4.0",
			},
			validate: func(t *testing.T, c v1beta1.Chart) {
				require.NotNil(t, c.Git)
				require.Equal(t, "https://github.com/example/repo@6.4.0", c.Git.URL)
				require.Equal(t, "charts/my-chart", c.Git.Path)
			},
		},
		{
			name: "git repo without version",
			chart: v1alpha1.ZarfChart{
				Name:    "my-chart",
				URL:     "https://github.com/example/repo",
				GitPath: "charts/my-chart",
			},
			validate: func(t *testing.T, c v1beta1.Chart) {
				require.NotNil(t, c.Git)
				require.Equal(t, "https://github.com/example/repo", c.Git.URL)
				require.Equal(t, "charts/my-chart", c.Git.Path)
			},
		},
		{
			name: "git repo with ref already in url",
			chart: v1alpha1.ZarfChart{
				Name:    "my-chart",
				URL:     "https://github.com/example/repo.git@v2.0.0",
				GitPath: "charts/my-chart",
				Version: "6.4.0",
			},
			validate: func(t *testing.T, c v1beta1.Chart) {
				require.NotNil(t, c.Git)
				// URL already has @ref, should not double-append version.
				require.Equal(t, "https://github.com/example/repo.git@v2.0.0", c.Git.URL)
				require.Equal(t, "charts/my-chart", c.Git.Path)
			},
		},
		{
			name: "local path",
			chart: v1alpha1.ZarfChart{
				Name:      "local-chart",
				LocalPath: "./charts/my-chart",
			},
			validate: func(t *testing.T, c v1beta1.Chart) {
				require.NotNil(t, c.Local)
				require.Equal(t, "./charts/my-chart", c.Local.Path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pkg := v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{
					{
						Name:   "chart-comp",
						Charts: []v1alpha1.ZarfChart{tt.chart},
					},
				},
			}
			result := V1Alpha1PkgToV1Beta1(pkg)
			require.Len(t, result.Components, 1)
			require.Len(t, result.Components[0].Charts, 1)
			tt.validate(t, result.Components[0].Charts[0])
		})
	}
}

func TestV1Alpha1PkgToV1Beta1_ManifestSkipWait(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "manifest-comp",
				Manifests: []v1alpha1.ZarfManifest{
					{
						Name:   "with-no-wait",
						NoWait: true,
					},
					{
						Name: "default-wait",
					},
				},
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Len(t, result.Components[0].Manifests, 2)

	// NoWait=true → SkipWait=true.
	require.True(t, result.Components[0].Manifests[0].SkipWait)
	// NoWait=false → SkipWait=false.
	require.False(t, result.Components[0].Manifests[1].SkipWait)
}

func TestV1Alpha1PkgToV1Beta1_Actions(t *testing.T) {
	t.Parallel()
	mute := true
	maxSeconds := 30
	maxRetries := 3
	dir := "/tmp"

	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "action-comp",
				Actions: v1alpha1.ZarfComponentActions{
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Mute:            true,
							MaxTotalSeconds: 60,
							MaxRetries:      2,
							Dir:             "/work",
							Env:             []string{"FOO=bar"},
							Shell: v1alpha1.Shell{
								Linux: "bash",
							},
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								Cmd:             "echo before",
								Mute:            &mute,
								MaxTotalSeconds: &maxSeconds,
								MaxRetries:      &maxRetries,
								Dir:             &dir,
								Description:     "run before",
								SetVariables: []v1alpha1.Variable{
									{Name: "OUT_VAR", Sensitive: true},
								},
								DeprecatedSetVariable: "OLD_VAR",
							},
						},
						After: []v1alpha1.ZarfComponentAction{
							{Cmd: "echo after"},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{Cmd: "echo success"},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{Cmd: "echo failure"},
						},
					},
				},
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Len(t, result.Components, 1)
	actions := result.Components[0].Actions

	// Defaults
	require.True(t, actions.OnDeploy.Defaults.Silent)
	require.Equal(t, int32(60), actions.OnDeploy.Defaults.MaxTotalSeconds)
	require.Equal(t, int32(2), actions.OnDeploy.Defaults.Retries)
	require.Equal(t, "/work", actions.OnDeploy.Defaults.Dir)
	require.Equal(t, []string{"FOO=bar"}, actions.OnDeploy.Defaults.Env)
	require.Equal(t, "bash", actions.OnDeploy.Defaults.Shell.Linux)

	// Before action
	require.Len(t, actions.OnDeploy.Before, 1)
	before := actions.OnDeploy.Before[0]
	require.Equal(t, "echo before", before.Cmd)
	require.NotNil(t, before.Silent)
	require.True(t, *before.Silent)
	require.NotNil(t, before.MaxTotalSeconds)
	require.Equal(t, int32(30), *before.MaxTotalSeconds)
	require.NotNil(t, before.Retries)
	require.Equal(t, int32(3), *before.Retries)
	require.Equal(t, "run before", before.Description)
	// SetVariables should include both the explicit one and the deprecated one, surfaced via the shim.
	setVars := before.GetDeprecatedSetVariables()
	require.Len(t, setVars, 2)
	require.Equal(t, "OUT_VAR", setVars[0].Name)
	require.True(t, setVars[0].Sensitive)
	require.Equal(t, "OLD_VAR", setVars[1].Name)

	// OnSuccess should be the merge of v1alpha1 After + OnSuccess.
	require.Len(t, actions.OnDeploy.OnSuccess, 2)
	require.Equal(t, "echo after", actions.OnDeploy.OnSuccess[0].Cmd)
	require.Equal(t, "echo success", actions.OnDeploy.OnSuccess[1].Cmd)

	// OnFailure
	require.Len(t, actions.OnDeploy.OnFailure, 1)
	require.Equal(t, "echo failure", actions.OnDeploy.OnFailure[0].Cmd)
}

func TestV1Alpha1PkgToV1Beta1_Files(t *testing.T) {
	t.Parallel()
	tmpl := true
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "file-comp",
				Files: []v1alpha1.ZarfFile{
					{
						Source:      "https://example.com/file.tar.gz",
						Shasum:      "deadbeef",
						Target:      "/opt/file.tar.gz",
						Executable:  true,
						Symlinks:    []string{"/usr/local/bin/file"},
						ExtractPath: "bin/file",
						Template:    &tmpl,
					},
				},
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Len(t, result.Components[0].Files, 1)
	f := result.Components[0].Files[0]
	require.Equal(t, "https://example.com/file.tar.gz", f.Source)
	require.Equal(t, "deadbeef", f.Checksum)
	require.Equal(t, "/opt/file.tar.gz", f.Destination)
	require.True(t, f.Executable)
	require.Equal(t, []string{"/usr/local/bin/file"}, f.Symlinks)
	require.Equal(t, "bin/file", f.ExtractPath)
	require.True(t, f.EnableTemplating)
}

func TestV1Alpha1PkgToV1Beta1_ValuesAndDocumentation(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Values: v1alpha1.ZarfValues{
			Files:  []string{"values.yaml"},
			Schema: "values.schema.json",
		},
		Documentation: map[string]string{
			"readme": "# Hello",
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Equal(t, []string{"values.yaml"}, result.Values.Files)
	require.Equal(t, "values.schema.json", result.Values.Schema)
	require.Equal(t, "# Hello", result.Documentation["readme"])
}

func TestV1Alpha1PkgToV1Beta1_DeprecatedVersionShim(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Components: []v1alpha1.ZarfComponent{
			{
				Name: "chart-comp",
				Charts: []v1alpha1.ZarfChart{
					{
						Name:    "my-chart",
						URL:     "https://charts.example.com",
						Version: "1.2.3",
					},
				},
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Len(t, result.Components[0].Charts, 1)
	chart := result.Components[0].Charts[0]
	require.Equal(t, "1.2.3", chart.GetDeprecatedVersion())
}

func TestV1Alpha1PkgToV1Beta1_OriginalAPIVersion(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.ZarfPackageConfig,
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Equal(t, v1alpha1.APIVersion, result.Build.GetOriginalAPIVersion())
}

// --- v1beta1 → v1alpha1 tests ---

func TestV1Beta1PkgToV1Alpha1_Metadata(t *testing.T) {
	t.Parallel()
	pkg := v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata: v1beta1.PackageMetadata{
			Name:                     "test-pkg",
			Description:              "A test package",
			Version:                  "1.0.0",
			Architecture:             "amd64",
			Uncompressed:             true,
			PreventNamespaceOverride: false,
			Annotations: map[string]string{
				"existing":               "annotation",
				"metadata.url":           "https://example.com",
				"metadata.image":         "https://example.com/image.png",
				"metadata.authors":       "Test Author",
				"metadata.documentation": "https://docs.example.com",
				"metadata.source":        "https://github.com/example",
				"metadata.vendor":        "Example Corp",
			},
		},
		Build: v1beta1.BuildData{
			AggregateChecksum: "abc123",
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Equal(t, v1alpha1.APIVersion, result.APIVersion)
	require.Equal(t, v1alpha1.ZarfPackageConfig, result.Kind)
	require.Equal(t, "test-pkg", result.Metadata.Name)
	require.Equal(t, "A test package", result.Metadata.Description)
	require.Equal(t, "1.0.0", result.Metadata.Version)
	require.Equal(t, "amd64", result.Metadata.Architecture)
	require.True(t, result.Metadata.Uncompressed)
	// PreventNamespaceOverride=false → AllowNamespaceOverride=true.
	require.NotNil(t, result.Metadata.AllowNamespaceOverride)
	require.True(t, *result.Metadata.AllowNamespaceOverride)

	// v1alpha1-only metadata fields should be restored from annotations.
	require.Equal(t, "https://example.com", result.Metadata.URL)
	require.Equal(t, "https://example.com/image.png", result.Metadata.Image)
	require.Equal(t, "Test Author", result.Metadata.Authors)
	require.Equal(t, "https://docs.example.com", result.Metadata.Documentation)
	require.Equal(t, "https://github.com/example", result.Metadata.Source)
	require.Equal(t, "Example Corp", result.Metadata.Vendor)

	// Metadata-specific annotations should be consumed, regular annotations preserved.
	require.Equal(t, "annotation", result.Metadata.Annotations["existing"])
	require.Empty(t, result.Metadata.Annotations["metadata.url"])

	// AggregateChecksum should move from build to metadata.
	require.Equal(t, "abc123", result.Metadata.AggregateChecksum)
}

func TestV1Beta1PkgToV1Alpha1_Build(t *testing.T) {
	t.Parallel()
	signed := true
	pkg := v1beta1.Package{
		Kind: v1beta1.ZarfPackageConfig,
		Build: v1beta1.BuildData{
			Hostname:                   "my-machine",
			User:                       "test-user",
			Architecture:               "arm64",
			Timestamp:                  "Mon, 02 Jan 2006 15:04:05 -0700",
			Version:                    "v0.30.0",
			Migrations:                 []string{"migration1"},
			RegistryOverrides:          map[string]string{"docker.io": "internal.registry"},
			Differential:               true,
			DifferentialPackageVersion: "0.29.0",
			Flavor:                     "vanilla",
			Signed:                     &signed,
			VersionRequirements: []v1beta1.VersionRequirement{
				{Version: "v0.28.0", Reason: "needs feature X"},
			},
			ProvenanceFiles: []string{"sig.json"},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Equal(t, v1alpha1.ZarfPackageConfig, result.Kind)
	require.Equal(t, "my-machine", result.Build.Terminal)
	require.Equal(t, "test-user", result.Build.User)
	require.Equal(t, "arm64", result.Build.Architecture)
	require.Equal(t, "v0.30.0", result.Build.Version)
	require.True(t, result.Build.Differential)
	require.Equal(t, "0.29.0", result.Build.DifferentialPackageVersion)
	require.Equal(t, "vanilla", result.Build.Flavor)
	require.NotNil(t, result.Build.Signed)
	require.True(t, *result.Build.Signed)
	require.Len(t, result.Build.VersionRequirements, 1)
	require.Equal(t, "v0.28.0", result.Build.VersionRequirements[0].Version)
}

func TestV1Beta1PkgToV1Alpha1_ComponentBasics(t *testing.T) {
	t.Parallel()
	pkg := v1beta1.Package{
		Kind: v1beta1.ZarfPackageConfig,
		Components: []v1beta1.Component{
			{
				Name:        "my-component",
				Description: "test component",
				Optional:    true,
				ComponentSpec: v1beta1.ComponentSpec{
					Target: v1beta1.ComponentTarget{
						OS:           "linux",
						Architecture: "amd64",
						Flavor:       "vanilla",
					},
					Import: v1beta1.ComponentImport{
						Local:  []v1beta1.ComponentImportLocal{{Path: "./path"}},
						Remote: []v1beta1.ComponentImportRemote{{URL: "oci://example.com/pkg"}},
					},
					Images: []v1beta1.Image{
						{Name: "nginx:latest"},
						{Name: "redis:7", Source: "daemon"},
					},
					Repositories: []v1beta1.Repository{
						{URL: "https://github.com/example/repo"},
						{URL: "https://github.com/example/other"},
					},
					StateAccess: []v1beta1.StateAccessKey{
						v1beta1.StateAccessRegistryCredentials,
						v1beta1.StateAccessAgentCerts,
					},
				},
			},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Len(t, result.Components, 1)
	comp := result.Components[0]
	require.Equal(t, "my-component", comp.Name)
	require.Equal(t, "test component", comp.Description)

	// Optional=true → Required=false.
	require.NotNil(t, comp.Required)
	require.False(t, *comp.Required)

	require.Equal(t, "linux", comp.Only.LocalOS)
	require.Equal(t, "amd64", comp.Only.Cluster.Architecture)
	require.Equal(t, "vanilla", comp.Only.Flavor)

	// v1beta1 Local[0] / Remote[0] project back onto v1alpha1 Import.Path/URL.
	require.Equal(t, "./path", comp.Import.Path)
	require.Equal(t, "oci://example.com/pkg", comp.Import.URL)

	// Images are converted from []Image back to []string.
	require.Len(t, comp.Images, 2)
	require.Equal(t, "nginx:latest", comp.Images[0])
	require.Equal(t, "redis:7", comp.Images[1])

	// v1beta1 Repository objects collapse back to v1alpha1 Repos ([]string of URLs).
	require.Equal(t, []string{
		"https://github.com/example/repo",
		"https://github.com/example/other",
	}, comp.Repos)

	// StateAccess keys carry over unchanged.
	require.Equal(t, []v1alpha1.StateAccessKey{
		v1alpha1.StateAccessRegistryCredentials,
		v1alpha1.StateAccessAgentCerts,
	}, comp.StateAccess)
}

func TestV1Beta1PkgToV1Alpha1_ChartSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		chart    v1beta1.Chart
		validate func(t *testing.T, c v1alpha1.ZarfChart)
	}{
		{
			name: "helm repo",
			chart: v1beta1.Chart{
				Name: "podinfo",
				HelmRepository: &v1beta1.HelmRepositorySource{
					URL:     "https://stefanprodan.github.io/podinfo",
					Name:    "podinfo",
					Version: "6.4.0",
				},
			},
			validate: func(t *testing.T, c v1alpha1.ZarfChart) {
				require.Equal(t, "https://stefanprodan.github.io/podinfo", c.URL)
				require.Equal(t, "podinfo", c.RepoName)
				require.Equal(t, "6.4.0", c.Version)
			},
		},
		{
			name: "oci registry",
			chart: v1beta1.Chart{
				Name: "podinfo",
				OCI: &v1beta1.OCISource{
					URL:     "oci://ghcr.io/stefanprodan/charts/podinfo",
					Version: "6.4.0",
				},
			},
			validate: func(t *testing.T, c v1alpha1.ZarfChart) {
				require.Equal(t, "oci://ghcr.io/stefanprodan/charts/podinfo", c.URL)
				require.Equal(t, "6.4.0", c.Version)
			},
		},
		{
			name: "git repo with version in url",
			chart: v1beta1.Chart{
				Name: "my-chart",
				Git: &v1beta1.GitSource{
					URL:  "https://github.com/example/repo@6.4.0",
					Path: "charts/my-chart",
				},
			},
			validate: func(t *testing.T, c v1alpha1.ZarfChart) {
				require.Equal(t, "https://github.com/example/repo", c.URL)
				require.Equal(t, "charts/my-chart", c.GitPath)
				require.Equal(t, "6.4.0", c.Version)
			},
		},
		{
			name: "git repo without version",
			chart: v1beta1.Chart{
				Name: "my-chart",
				Git: &v1beta1.GitSource{
					URL:  "https://github.com/example/repo",
					Path: "charts/my-chart",
				},
			},
			validate: func(t *testing.T, c v1alpha1.ZarfChart) {
				require.Equal(t, "https://github.com/example/repo", c.URL)
				require.Equal(t, "charts/my-chart", c.GitPath)
			},
		},
		{
			name: "local path",
			chart: v1beta1.Chart{
				Name:  "local-chart",
				Local: &v1beta1.LocalSource{Path: "./charts/my-chart"},
			},
			validate: func(t *testing.T, c v1alpha1.ZarfChart) {
				require.Equal(t, "./charts/my-chart", c.LocalPath)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pkg := v1beta1.Package{
				Kind: v1beta1.ZarfPackageConfig,
				Components: []v1beta1.Component{
					{
						Name:          "chart-comp",
						ComponentSpec: v1beta1.ComponentSpec{Charts: []v1beta1.Chart{tt.chart}},
					},
				},
			}
			result := V1Beta1PkgToV1Alpha1(pkg)
			require.Len(t, result.Components, 1)
			require.Len(t, result.Components[0].Charts, 1)
			tt.validate(t, result.Components[0].Charts[0])
		})
	}
}

func TestV1Beta1PkgToV1Alpha1_ManifestSkipWaitInversion(t *testing.T) {
	t.Parallel()
	pkg := v1beta1.Package{
		Kind: v1beta1.ZarfPackageConfig,
		Components: []v1beta1.Component{
			{
				Name: "manifest-comp",
				ComponentSpec: v1beta1.ComponentSpec{
					Manifests: []v1beta1.Manifest{
						{
							Name:     "skip-wait",
							SkipWait: true,
						},
						{
							Name: "default-wait",
						},
					},
				},
			},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Len(t, result.Components[0].Manifests, 2)

	// SkipWait=true → NoWait=true.
	require.True(t, result.Components[0].Manifests[0].NoWait)
	// SkipWait=false → NoWait=false.
	require.False(t, result.Components[0].Manifests[1].NoWait)
}

func TestV1Beta1PkgToV1Alpha1_Actions(t *testing.T) {
	t.Parallel()
	mute := true
	dir := "/tmp"
	maxSec := int32(30)
	retries := int32(3)
	maxSecDef := int32(60)
	retriesDef := int32(2)

	pkg := v1beta1.Package{
		Kind: v1beta1.ZarfPackageConfig,
		Components: []v1beta1.Component{
			{
				Name: "action-comp",
				ComponentSpec: v1beta1.ComponentSpec{
					Actions: v1beta1.ComponentActions{
						OnDeploy: v1beta1.ComponentActionSet{
							Defaults: v1beta1.ComponentActionDefaults{
								Silent:          true,
								MaxTotalSeconds: maxSecDef,
								Retries:         retriesDef,
								Dir:             "/work",
								Env:             []string{"FOO=bar"},
								Shell: v1beta1.Shell{
									Linux: "bash",
								},
							},
							Before: []v1beta1.ComponentAction{
								{
									Cmd:             "echo before",
									Silent:          &mute,
									MaxTotalSeconds: &maxSec,
									Retries:         &retries,
									Dir:             &dir,
								},
							},
							OnSuccess: []v1beta1.ComponentAction{
								{Cmd: "echo after"},
							},
							OnFailure: []v1beta1.ComponentAction{
								{Cmd: "echo failure"},
							},
						},
					},
				},
			},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Len(t, result.Components, 1)
	actions := result.Components[0].Actions

	// Defaults
	require.True(t, actions.OnDeploy.Defaults.Mute)
	require.Equal(t, 60, actions.OnDeploy.Defaults.MaxTotalSeconds)
	require.Equal(t, 2, actions.OnDeploy.Defaults.MaxRetries)
	require.Equal(t, "/work", actions.OnDeploy.Defaults.Dir)
	require.Equal(t, []string{"FOO=bar"}, actions.OnDeploy.Defaults.Env)
	require.Equal(t, "bash", actions.OnDeploy.Defaults.Shell.Linux)

	// Before action
	require.Len(t, actions.OnDeploy.Before, 1)
	before := actions.OnDeploy.Before[0]
	require.Equal(t, "echo before", before.Cmd)
	require.NotNil(t, before.Mute)
	require.True(t, *before.Mute)
	require.NotNil(t, before.MaxTotalSeconds)
	require.Equal(t, 30, *before.MaxTotalSeconds)
	require.NotNil(t, before.MaxRetries)
	require.Equal(t, 3, *before.MaxRetries)

	// OnSuccess
	require.Len(t, actions.OnDeploy.OnSuccess, 1)
	require.Equal(t, "echo after", actions.OnDeploy.OnSuccess[0].Cmd)

	// OnFailure
	require.Len(t, actions.OnDeploy.OnFailure, 1)
	require.Equal(t, "echo failure", actions.OnDeploy.OnFailure[0].Cmd)
}

func TestV1Beta1PkgToV1Alpha1_VariablesShim(t *testing.T) {
	t.Parallel()
	pkg := v1beta1.SetDeprecatedFromGeneric(types.Package{
		Variables: []types.InteractiveVariable{
			{
				Variable: types.Variable{
					Name:       "MY_VAR",
					Sensitive:  true,
					AutoIndent: true,
					Pattern:    "^[a-z]+$",
					Type:       types.FileVariableType,
				},
				Description: "A variable",
				Default:     "default-val",
				Prompt:      true,
			},
		},
		Constants: []types.Constant{
			{Name: "MY_CONST", Value: "const-val"},
		},
	}, v1beta1.Package{Kind: v1beta1.ZarfPackageConfig})

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Len(t, result.Variables, 1)
	v := result.Variables[0]
	require.Equal(t, "MY_VAR", v.Name)
	require.True(t, v.Sensitive)
	require.Equal(t, v1alpha1.FileVariableType, v.Type)
	require.Equal(t, "A variable", v.Description)
	require.Equal(t, "default-val", v.Default)
	require.True(t, v.Prompt)

	require.Len(t, result.Constants, 1)
	require.Equal(t, "MY_CONST", result.Constants[0].Name)
	require.Equal(t, "const-val", result.Constants[0].Value)
}

func TestV1Beta1PkgToV1Alpha1_OriginalAPIVersion(t *testing.T) {
	t.Parallel()
	pkg := v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Equal(t, v1beta1.APIVersion, result.Build.OriginalAPIVersion())
}

func TestOriginalAPIVersion_SurvivesRoundTrip(t *testing.T) {
	t.Parallel()
	pkg := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.ZarfPackageConfig,
	}

	beta := V1Alpha1PkgToV1Beta1(pkg)
	require.Equal(t, v1alpha1.APIVersion, beta.Build.GetOriginalAPIVersion())

	// Converting back must preserve the true original, not report v1beta1.
	result := V1Beta1PkgToV1Alpha1(beta)
	require.Equal(t, v1alpha1.APIVersion, result.Build.OriginalAPIVersion())
}

func TestRoundTrip_V1Alpha1_To_V1Beta1_And_Back(t *testing.T) {
	t.Parallel()
	required := true
	allowOverride := true
	mute := true
	maxSeconds := 30
	maxRetries := 3
	dir := "/tmp"

	original := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name:                   "round-trip-pkg",
			Description:            "round trip test",
			Version:                "2.0.0",
			Architecture:           "arm64",
			URL:                    "https://example.com",
			Authors:                "Test Author",
			AllowNamespaceOverride: &allowOverride,
		},
		Build: v1alpha1.ZarfBuildData{
			Terminal:     "my-machine",
			Architecture: "arm64",
			Timestamp:    "Mon, 02 Jan 2006 15:04:05 -0700",
			Version:      "v0.30.0",
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name:     "test-comp",
				Required: &required,
				Images:   []string{"nginx:latest"},
				Repos:    []string{"https://github.com/example/repo", "https://github.com/example/other"},
				StateAccess: []v1alpha1.StateAccessKey{
					v1alpha1.StateAccessRegistryCredentials,
					v1alpha1.StateAccessGitCredentials,
					v1alpha1.StateAccessAgentCerts,
				},
				Charts: []v1alpha1.ZarfChart{
					{
						Name:      "my-chart",
						URL:       "https://charts.example.com",
						RepoName:  "my-chart",
						Version:   "1.2.3",
						Namespace: "default",
					},
				},
				Manifests: []v1alpha1.ZarfManifest{
					{
						Name:      "my-manifest",
						Namespace: "default",
						Files:     []string{"manifest.yaml"},
						NoWait:    true,
					},
				},
				Actions: v1alpha1.ZarfComponentActions{
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							MaxTotalSeconds: 60,
							MaxRetries:      2,
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								Cmd:             "echo before",
								Mute:            &mute,
								MaxTotalSeconds: &maxSeconds,
								MaxRetries:      &maxRetries,
								Dir:             &dir,
							},
						},
					},
				},
				DataInjections: []v1alpha1.ZarfDataInjection{
					{Source: "/data", Target: v1alpha1.ZarfContainerTarget{Namespace: "default", Selector: "app=test", Container: "main", Path: "/inject"}},
				},
			},
		},
		Constants: []v1alpha1.Constant{
			{Name: "MY_CONST", Value: "val"},
		},
		Variables: []v1alpha1.InteractiveVariable{
			{
				Variable:    v1alpha1.Variable{Name: "MY_VAR"},
				Description: "a var",
				Default:     "default",
			},
		},
	}

	// Round-trip: v1alpha1 → v1beta1 → v1alpha1.
	beta := V1Alpha1PkgToV1Beta1(original)
	result := V1Beta1PkgToV1Alpha1(beta)

	require.Equal(t, v1alpha1.APIVersion, result.APIVersion)
	require.Equal(t, original.Kind, result.Kind)
	require.Equal(t, original.Metadata.Name, result.Metadata.Name)
	require.Equal(t, original.Metadata.Description, result.Metadata.Description)
	require.Equal(t, original.Metadata.Version, result.Metadata.Version)
	require.Equal(t, original.Metadata.Architecture, result.Metadata.Architecture)
	require.Equal(t, original.Metadata.URL, result.Metadata.URL)
	require.Equal(t, original.Metadata.Authors, result.Metadata.Authors)

	require.Len(t, result.Components, 1)
	comp := result.Components[0]
	require.Equal(t, "test-comp", comp.Name)
	require.NotNil(t, comp.Required)
	require.True(t, *comp.Required)
	require.Equal(t, []string{"nginx:latest"}, comp.Images)

	// Repos and StateAccess should survive the round-trip unchanged.
	require.Equal(t, []string{"https://github.com/example/repo", "https://github.com/example/other"}, comp.Repos)
	require.Equal(t, []v1alpha1.StateAccessKey{
		v1alpha1.StateAccessRegistryCredentials,
		v1alpha1.StateAccessGitCredentials,
		v1alpha1.StateAccessAgentCerts,
	}, comp.StateAccess)

	// Chart should round-trip via structured source → flat fields.
	require.Len(t, comp.Charts, 1)
	require.Equal(t, "https://charts.example.com", comp.Charts[0].URL)
	require.Equal(t, "my-chart", comp.Charts[0].RepoName)
	require.Equal(t, "1.2.3", comp.Charts[0].Version)

	// Manifest NoWait should survive round-trip.
	require.Len(t, comp.Manifests, 1)
	require.True(t, comp.Manifests[0].NoWait)

	// Action timeouts should round-trip through int32 conversion.
	require.Equal(t, 60, comp.Actions.OnDeploy.Defaults.MaxTotalSeconds)
	require.Equal(t, 2, comp.Actions.OnDeploy.Defaults.MaxRetries)
	require.Len(t, comp.Actions.OnDeploy.Before, 1)
	require.NotNil(t, comp.Actions.OnDeploy.Before[0].MaxTotalSeconds)
	require.Equal(t, 30, *comp.Actions.OnDeploy.Before[0].MaxTotalSeconds)

	// DataInjections should survive round-trip via private shim.
	require.Len(t, comp.DataInjections, 1)
	require.Equal(t, "/data", comp.DataInjections[0].Source)

	// Constants and variables.
	require.Len(t, result.Constants, 1)
	require.Equal(t, "MY_CONST", result.Constants[0].Name)
	require.Len(t, result.Variables, 1)
	require.Equal(t, "MY_VAR", result.Variables[0].Name)
}
