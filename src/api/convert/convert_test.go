// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package convert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	require.NotNil(t, result.Metadata.AllowNamespaceOverride)
	require.True(t, *result.Metadata.AllowNamespaceOverride)

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
		Kind: v1alpha1.ZarfInitConfig,
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
			APIVersion:                 v1alpha1.APIVersion,
			VersionRequirements: []v1alpha1.VersionRequirement{
				{Version: "v0.28.0", Reason: "needs feature X"},
			},
			ProvenanceFiles: []string{"sig.json"},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Equal(t, v1beta1.ZarfInitConfig, result.Kind)
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
	require.Equal(t, "needs feature X", result.Build.VersionRequirements[0].Reason)
	require.Equal(t, []string{"sig.json"}, result.Build.ProvenanceFiles)
	require.Equal(t, v1alpha1.APIVersion, result.Build.APIVersion)
}

func TestV1Alpha1PkgToV1Beta1_Variables(t *testing.T) {
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

	require.Len(t, result.Variables, 1)
	v := result.Variables[0]
	require.Equal(t, "MY_VAR", v.Name)
	require.True(t, v.Sensitive)
	require.True(t, v.AutoIndent)
	require.Equal(t, "^[a-z]+$", v.Pattern)
	require.Equal(t, v1beta1.FileVariableType, v.Type)
	require.Equal(t, "A variable", v.Description)
	require.Equal(t, "default-val", v.Default)
	require.True(t, v.Prompt)

	require.Len(t, result.Constants, 1)
	c := result.Constants[0]
	require.Equal(t, "MY_CONST", c.Name)
	require.Equal(t, "const-val", c.Value)
	require.Equal(t, "A constant", c.Description)
	require.True(t, c.AutoIndent)
}

func TestV1Alpha1PkgToV1Beta1_ComponentBasics(t *testing.T) {
	t.Parallel()
	required := true
	pkg := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Components: []v1alpha1.ZarfComponent{
			{
				Name:            "my-component",
				Description:     "test component",
				Default:         true,
				Required:        &required,
				DeprecatedGroup: "my-group",
				Only: v1alpha1.ZarfComponentOnlyTarget{
					LocalOS: "linux",
					Cluster: v1alpha1.ZarfComponentOnlyCluster{
						Architecture: "amd64",
						Distros:      []string{"k3s"},
					},
					Flavor: "vanilla",
				},
				Import: v1alpha1.ZarfComponentImport{
					Name: "imported",
					Path: "./path",
					URL:  "oci://example.com/pkg",
				},
				Images: []string{"nginx:latest", "redis:7"},
				Repos:  []string{"https://github.com/example/repo"},
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
	require.True(t, comp.Default)

	// Required=true → Optional=false
	require.NotNil(t, comp.Optional)
	require.False(t, *comp.Optional)

	require.Equal(t, "linux", comp.Only.LocalOS)
	require.Equal(t, "amd64", comp.Only.Cluster.Architecture)
	require.Equal(t, []string{"k3s"}, comp.Only.Cluster.Distros)
	require.Equal(t, "vanilla", comp.Only.Flavor)

	// Import.Name is v1alpha1-only, should be dropped in v1beta1.
	require.Equal(t, "./path", comp.Import.Path)
	require.Equal(t, "oci://example.com/pkg", comp.Import.URL)

	// Images are converted from []string to []ZarfImage.
	require.Len(t, comp.Images, 2)
	require.Equal(t, "nginx:latest", comp.Images[0].Name)
	require.Equal(t, "redis:7", comp.Images[1].Name)

	require.Equal(t, []string{"https://github.com/example/repo"}, comp.Repos)

	// DataInjections should be preserved via the private shim.
	require.Len(t, comp.GetDataInjections(), 1)
	require.Equal(t, "/data", comp.GetDataInjections()[0].Source)

	// HealthChecks should become onDeploy after wait actions with kind in <kind>.<version>.<group> format.
	require.Len(t, comp.Actions.OnDeploy.After, 2)
	require.NotNil(t, comp.Actions.OnDeploy.After[0].Wait)
	require.NotNil(t, comp.Actions.OnDeploy.After[0].Wait.Cluster)
	require.Equal(t, "Deployment.v1.apps", comp.Actions.OnDeploy.After[0].Wait.Cluster.Kind)
	require.Equal(t, "my-deploy", comp.Actions.OnDeploy.After[0].Wait.Cluster.Name)
	require.Equal(t, "default", comp.Actions.OnDeploy.After[0].Wait.Cluster.Namespace)

	// Core API resources (no group) keep kind as-is.
	require.NotNil(t, comp.Actions.OnDeploy.After[1].Wait)
	require.NotNil(t, comp.Actions.OnDeploy.After[1].Wait.Cluster)
	require.Equal(t, "Pod", comp.Actions.OnDeploy.After[1].Wait.Cluster.Kind)
	require.Equal(t, "my-pod", comp.Actions.OnDeploy.After[1].Wait.Cluster.Name)
	require.Equal(t, "default", comp.Actions.OnDeploy.After[1].Wait.Cluster.Namespace)
}

func TestV1Alpha1PkgToV1Beta1_FeatureInference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		compName   string
		isRegistry bool
		isAgent    bool
		injector   bool
	}{
		{name: "zarf-registry", compName: "zarf-registry", isRegistry: true, isAgent: false, injector: false},
		{name: "zarf-injector", compName: "zarf-injector", isRegistry: true, isAgent: false, injector: false},
		{name: "zarf-seed-registry", compName: "zarf-seed-registry", isRegistry: true, isAgent: false, injector: true},
		{name: "zarf-agent", compName: "zarf-agent", isRegistry: false, isAgent: true, injector: false},
		{name: "regular component", compName: "my-app", isRegistry: false, isAgent: false, injector: false},
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
			comp := result.Components[0]
			require.Equal(t, tt.isRegistry, comp.Features.IsRegistry, "IsRegistry")
			require.Equal(t, tt.isAgent, comp.Features.IsAgent, "IsAgent")
			if tt.injector {
				require.NotNil(t, comp.Features.Injector)
				require.True(t, comp.Features.Injector.Enabled)
			} else {
				require.Nil(t, comp.Features.Injector)
			}
		})
	}
}

func TestV1Alpha1PkgToV1Beta1_ChartSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		chart    v1alpha1.ZarfChart
		validate func(t *testing.T, c v1beta1.ZarfChart)
	}{
		{
			name: "helm repo",
			chart: v1alpha1.ZarfChart{
				Name:     "podinfo",
				URL:      "https://stefanprodan.github.io/podinfo",
				RepoName: "podinfo",
				Version:  "6.4.0",
			},
			validate: func(t *testing.T, c v1beta1.ZarfChart) {
				require.Equal(t, "https://stefanprodan.github.io/podinfo", c.HelmRepo.URL)
				require.Equal(t, "podinfo", c.HelmRepo.Name)
				require.Equal(t, "6.4.0", c.HelmRepo.Version)
			},
		},
		{
			name: "oci registry",
			chart: v1alpha1.ZarfChart{
				Name:    "podinfo",
				URL:     "oci://ghcr.io/stefanprodan/charts/podinfo",
				Version: "6.4.0",
			},
			validate: func(t *testing.T, c v1beta1.ZarfChart) {
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
			validate: func(t *testing.T, c v1beta1.ZarfChart) {
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
			validate: func(t *testing.T, c v1beta1.ZarfChart) {
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
			validate: func(t *testing.T, c v1beta1.ZarfChart) {
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
			validate: func(t *testing.T, c v1beta1.ZarfChart) {
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

func TestV1Alpha1PkgToV1Beta1_ManifestNoWaitInversion(t *testing.T) {
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

	// NoWait=true → Wait=false
	m0 := result.Components[0].Manifests[0]
	require.NotNil(t, m0.Wait)
	require.False(t, *m0.Wait)

	// NoWait=false (default) → Wait=nil (v1beta1 defaults to true)
	m1 := result.Components[0].Manifests[1]
	require.Nil(t, m1.Wait)
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
	require.True(t, actions.OnDeploy.Defaults.Mute)
	require.NotNil(t, actions.OnDeploy.Defaults.Timeout)
	require.Equal(t, 60*time.Second, actions.OnDeploy.Defaults.Timeout.Duration)
	require.Equal(t, 2, actions.OnDeploy.Defaults.Retries)
	require.Equal(t, "/work", actions.OnDeploy.Defaults.Dir)
	require.Equal(t, []string{"FOO=bar"}, actions.OnDeploy.Defaults.Env)
	require.Equal(t, "bash", actions.OnDeploy.Defaults.Shell.Linux)

	// Before action
	require.Len(t, actions.OnDeploy.Before, 1)
	before := actions.OnDeploy.Before[0]
	require.Equal(t, "echo before", before.Cmd)
	require.NotNil(t, before.Mute)
	require.True(t, *before.Mute)
	require.NotNil(t, before.Timeout)
	require.Equal(t, 30*time.Second, before.Timeout.Duration)
	require.Equal(t, 3, before.Retries)
	require.Equal(t, "run before", before.Description)
	// SetVariables should include both the explicit one and the deprecated one.
	require.Len(t, before.SetVariables, 2)
	require.Equal(t, "OUT_VAR", before.SetVariables[0].Name)
	require.True(t, before.SetVariables[0].Sensitive)
	require.Equal(t, "OLD_VAR", before.SetVariables[1].Name)

	// After should include original After + OnSuccess (merged).
	require.Len(t, actions.OnDeploy.After, 2)
	require.Equal(t, "echo after", actions.OnDeploy.After[0].Cmd)
	require.Equal(t, "echo success", actions.OnDeploy.After[1].Cmd)

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
	require.Equal(t, "deadbeef", f.Shasum)
	require.Equal(t, "/opt/file.tar.gz", f.Target)
	require.True(t, f.Executable)
	require.Equal(t, []string{"/usr/local/bin/file"}, f.Symlinks)
	require.Equal(t, "bin/file", f.ExtractPath)
	require.NotNil(t, f.Template)
	require.True(t, *f.Template)
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

// --- v1beta1 → v1alpha1 tests ---

func TestV1Beta1PkgToV1Alpha1_Metadata(t *testing.T) {
	t.Parallel()
	allowOverride := true
	pkg := v1beta1.ZarfPackage{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata: v1beta1.ZarfMetadata{
			Name:                   "test-pkg",
			Description:            "A test package",
			Version:                "1.0.0",
			Architecture:           "amd64",
			Uncompressed:           true,
			AllowNamespaceOverride: &allowOverride,
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
		Build: v1beta1.ZarfBuildData{
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
	pkg := v1beta1.ZarfPackage{
		Kind: v1beta1.ZarfInitConfig,
		Build: v1beta1.ZarfBuildData{
			Terminal:                   "my-machine",
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
			APIVersion:                 v1beta1.APIVersion,
			VersionRequirements: []v1beta1.VersionRequirement{
				{Version: "v0.28.0", Reason: "needs feature X"},
			},
			ProvenanceFiles: []string{"sig.json"},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Equal(t, v1alpha1.ZarfInitConfig, result.Kind)
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
	require.Equal(t, v1beta1.APIVersion, result.Build.APIVersion)
}

func TestV1Beta1PkgToV1Alpha1_ComponentBasics(t *testing.T) {
	t.Parallel()
	optional := true
	pkg := v1beta1.ZarfPackage{
		Kind: v1beta1.ZarfPackageConfig,
		Components: []v1beta1.ZarfComponent{
			{
				Name:        "my-component",
				Description: "test component",
				Default:     true,
				Optional:    &optional,
				Only: v1beta1.ZarfComponentOnlyTarget{
					LocalOS: "linux",
					Cluster: v1beta1.ZarfComponentOnlyCluster{
						Architecture: "amd64",
						Distros:      []string{"k3s"},
					},
					Flavor: "vanilla",
				},
				Import: v1beta1.ZarfComponentImport{
					Path: "./path",
					URL:  "oci://example.com/pkg",
				},
				Images: []v1beta1.ZarfImage{
					{Name: "nginx:latest"},
					{Name: "redis:7", Source: "daemon"},
				},
				Repos: []string{"https://github.com/example/repo"},
			},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Len(t, result.Components, 1)
	comp := result.Components[0]
	require.Equal(t, "my-component", comp.Name)
	require.Equal(t, "test component", comp.Description)
	require.True(t, comp.Default)

	// Optional=true → Required=false
	require.NotNil(t, comp.Required)
	require.False(t, *comp.Required)

	require.Equal(t, "linux", comp.Only.LocalOS)
	require.Equal(t, "amd64", comp.Only.Cluster.Architecture)
	require.Equal(t, []string{"k3s"}, comp.Only.Cluster.Distros)
	require.Equal(t, "vanilla", comp.Only.Flavor)

	require.Equal(t, "./path", comp.Import.Path)
	require.Equal(t, "oci://example.com/pkg", comp.Import.URL)

	// Images are converted from []ZarfImage to []string.
	require.Len(t, comp.Images, 2)
	require.Equal(t, "nginx:latest", comp.Images[0])
	require.Equal(t, "redis:7", comp.Images[1])

	require.Equal(t, []string{"https://github.com/example/repo"}, comp.Repos)
}

func TestV1Beta1PkgToV1Alpha1_ChartSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		chart    v1beta1.ZarfChart
		validate func(t *testing.T, c v1alpha1.ZarfChart)
	}{
		{
			name: "helm repo",
			chart: v1beta1.ZarfChart{
				Name: "podinfo",
				HelmRepo: v1beta1.HelmRepoSource{
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
			chart: v1beta1.ZarfChart{
				Name: "podinfo",
				OCI: v1beta1.OCISource{
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
			chart: v1beta1.ZarfChart{
				Name: "my-chart",
				Git: v1beta1.GitRepoSource{
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
			chart: v1beta1.ZarfChart{
				Name: "my-chart",
				Git: v1beta1.GitRepoSource{
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
			chart: v1beta1.ZarfChart{
				Name: "local-chart",
				Local: v1beta1.LocalRepoSource{
					Path: "./charts/my-chart",
				},
			},
			validate: func(t *testing.T, c v1alpha1.ZarfChart) {
				require.Equal(t, "./charts/my-chart", c.LocalPath)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pkg := v1beta1.ZarfPackage{
				Kind: v1beta1.ZarfPackageConfig,
				Components: []v1beta1.ZarfComponent{
					{
						Name:   "chart-comp",
						Charts: []v1beta1.ZarfChart{tt.chart},
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

func TestV1Beta1PkgToV1Alpha1_ManifestWaitInversion(t *testing.T) {
	t.Parallel()
	waitFalse := false
	pkg := v1beta1.ZarfPackage{
		Kind: v1beta1.ZarfPackageConfig,
		Components: []v1beta1.ZarfComponent{
			{
				Name: "manifest-comp",
				Manifests: []v1beta1.ZarfManifest{
					{
						Name: "wait-false",
						Wait: &waitFalse,
					},
					{
						Name: "default-wait",
					},
				},
			},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Len(t, result.Components[0].Manifests, 2)

	// Wait=false → NoWait=true
	require.True(t, result.Components[0].Manifests[0].NoWait)

	// Wait=nil → NoWait=false (default)
	require.False(t, result.Components[0].Manifests[1].NoWait)
}

func TestV1Beta1PkgToV1Alpha1_Actions(t *testing.T) {
	t.Parallel()
	mute := true
	dir := "/tmp"

	pkg := v1beta1.ZarfPackage{
		Kind: v1beta1.ZarfPackageConfig,
		Components: []v1beta1.ZarfComponent{
			{
				Name: "action-comp",
				Actions: v1beta1.ZarfComponentActions{
					OnDeploy: v1beta1.ZarfComponentActionSet{
						Defaults: v1beta1.ZarfComponentActionDefaults{
							Mute:    true,
							Timeout: &metav1.Duration{Duration: 60 * time.Second},
							Retries: 2,
							Dir:     "/work",
							Env:     []string{"FOO=bar"},
							Shell: v1beta1.Shell{
								Linux: "bash",
							},
						},
						Before: []v1beta1.ZarfComponentAction{
							{
								Cmd:     "echo before",
								Mute:    &mute,
								Timeout: &metav1.Duration{Duration: 30 * time.Second},
								Retries: 3,
								Dir:     &dir,
								SetVariables: []v1beta1.Variable{
									{Name: "OUT_VAR", Sensitive: true},
								},
							},
						},
						After: []v1beta1.ZarfComponentAction{
							{Cmd: "echo after"},
						},
						OnFailure: []v1beta1.ZarfComponentAction{
							{Cmd: "echo failure"},
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
	require.Len(t, before.SetVariables, 1)
	require.Equal(t, "OUT_VAR", before.SetVariables[0].Name)
	require.True(t, before.SetVariables[0].Sensitive)

	// After
	require.Len(t, actions.OnDeploy.After, 1)
	require.Equal(t, "echo after", actions.OnDeploy.After[0].Cmd)

	// OnFailure
	require.Len(t, actions.OnDeploy.OnFailure, 1)
	require.Equal(t, "echo failure", actions.OnDeploy.OnFailure[0].Cmd)
}

func TestV1Beta1PkgToV1Alpha1_Variables(t *testing.T) {
	t.Parallel()
	pkg := v1beta1.ZarfPackage{
		Kind: v1beta1.ZarfPackageConfig,
		Variables: []v1beta1.InteractiveVariable{
			{
				Variable: v1beta1.Variable{
					Name:       "MY_VAR",
					Sensitive:  true,
					AutoIndent: true,
					Pattern:    "^[a-z]+$",
					Type:       v1beta1.FileVariableType,
				},
				Description: "A variable",
				Default:     "default-val",
				Prompt:      true,
			},
		},
		Constants: []v1beta1.Constant{
			{
				Name:        "MY_CONST",
				Value:       "const-val",
				Description: "A constant",
				AutoIndent:  true,
				Pattern:     ".*",
			},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	require.Len(t, result.Variables, 1)
	v := result.Variables[0]
	require.Equal(t, "MY_VAR", v.Name)
	require.True(t, v.Sensitive)
	require.True(t, v.AutoIndent)
	require.Equal(t, "^[a-z]+$", v.Pattern)
	require.Equal(t, v1alpha1.FileVariableType, v.Type)
	require.Equal(t, "A variable", v.Description)
	require.Equal(t, "default-val", v.Default)
	require.True(t, v.Prompt)

	require.Len(t, result.Constants, 1)
	c := result.Constants[0]
	require.Equal(t, "MY_CONST", c.Name)
	require.Equal(t, "const-val", c.Value)
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
				Repos:    []string{"https://github.com/example/repo"},
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

	// Round-trip: v1alpha1 → v1beta1 → v1alpha1
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

	// Chart should round-trip via structured source → flat fields.
	require.Len(t, comp.Charts, 1)
	require.Equal(t, "https://charts.example.com", comp.Charts[0].URL)
	require.Equal(t, "my-chart", comp.Charts[0].RepoName)
	require.Equal(t, "1.2.3", comp.Charts[0].Version)

	// Manifest NoWait should survive round-trip.
	require.Len(t, comp.Manifests, 1)
	require.True(t, comp.Manifests[0].NoWait)

	// Action timeouts should round-trip through Duration conversion.
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
