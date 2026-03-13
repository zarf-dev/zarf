// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package convert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	assert.Equal(t, v1beta1.APIVersion, result.APIVersion)
	assert.Equal(t, v1beta1.ZarfPackageConfig, result.Kind)
	assert.Equal(t, "test-pkg", result.Metadata.Name)
	assert.Equal(t, "A test package", result.Metadata.Description)
	assert.Equal(t, "1.0.0", result.Metadata.Version)
	assert.Equal(t, "amd64", result.Metadata.Architecture)
	assert.True(t, result.Metadata.Uncompressed)
	assert.True(t, result.Metadata.AllowNamespaceOverride)

	// v1alpha1-only metadata fields should be migrated to annotations.
	assert.Equal(t, "https://example.com", result.Metadata.Annotations["metadata.url"])
	assert.Equal(t, "https://example.com/image.png", result.Metadata.Annotations["metadata.image"])
	assert.Equal(t, "Test Author", result.Metadata.Annotations["metadata.authors"])
	assert.Equal(t, "https://docs.example.com", result.Metadata.Annotations["metadata.documentation"])
	assert.Equal(t, "https://github.com/example", result.Metadata.Annotations["metadata.source"])
	assert.Equal(t, "Example Corp", result.Metadata.Annotations["metadata.vendor"])
	// Existing annotation should be preserved.
	assert.Equal(t, "annotation", result.Metadata.Annotations["existing"])

	// AggregateChecksum should move from metadata to build.
	assert.Equal(t, "abc123", result.Build.AggregateChecksum)
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
			VersionRequirements: []v1alpha1.VersionRequirement{
				{Version: "v0.28.0", Reason: "needs feature X"},
			},
			ProvenanceFiles: []string{"sig.json"},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	assert.Equal(t, v1beta1.ZarfInitConfig, result.Kind)
	assert.Equal(t, "my-machine", result.Build.Terminal)
	assert.Equal(t, "test-user", result.Build.User)
	assert.Equal(t, "arm64", result.Build.Architecture)
	assert.Equal(t, "v0.30.0", result.Build.Version)
	assert.True(t, result.Build.Differential)
	assert.Equal(t, "0.29.0", result.Build.DifferentialPackageVersion)
	assert.Equal(t, "vanilla", result.Build.Flavor)
	require.NotNil(t, result.Build.Signed)
	assert.True(t, *result.Build.Signed)
	require.Len(t, result.Build.VersionRequirements, 1)
	assert.Equal(t, "v0.28.0", result.Build.VersionRequirements[0].Version)
	assert.Equal(t, "needs feature X", result.Build.VersionRequirements[0].Reason)
	assert.Equal(t, []string{"sig.json"}, result.Build.ProvenanceFiles)
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
	assert.Equal(t, "MY_VAR", v.Name)
	assert.True(t, v.Sensitive)
	assert.True(t, v.AutoIndent)
	assert.Equal(t, "^[a-z]+$", v.Pattern)
	assert.Equal(t, v1beta1.FileVariableType, v.Type)
	assert.Equal(t, "A variable", v.Description)
	assert.Equal(t, "default-val", v.Default)
	assert.True(t, v.Prompt)

	require.Len(t, result.Constants, 1)
	c := result.Constants[0]
	assert.Equal(t, "MY_CONST", c.Name)
	assert.Equal(t, "const-val", c.Value)
	assert.Equal(t, "A constant", c.Description)
	assert.True(t, c.AutoIndent)
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
				},
			},
		},
	}

	result := V1Alpha1PkgToV1Beta1(pkg)

	require.Len(t, result.Components, 1)
	comp := result.Components[0]
	assert.Equal(t, "my-component", comp.Name)
	assert.Equal(t, "test component", comp.Description)

	// Required=true → Optional=false
	require.NotNil(t, comp.Optional)
	assert.False(t, *comp.Optional)

	assert.Equal(t, "linux", comp.Only.LocalOS)
	assert.Equal(t, "amd64", comp.Only.Cluster.Architecture)
	assert.Equal(t, []string{"k3s"}, comp.Only.Cluster.Distros)
	assert.Equal(t, "vanilla", comp.Only.Flavor)

	// Import.Name is v1alpha1-only, should be dropped in v1beta1.
	assert.Equal(t, "./path", comp.Import.Path)
	assert.Equal(t, "oci://example.com/pkg", comp.Import.URL)

	// Images are converted from []string to []ZarfImage.
	require.Len(t, comp.Images, 2)
	assert.Equal(t, "nginx:latest", comp.Images[0].Name)
	assert.Equal(t, "redis:7", comp.Images[1].Name)

	assert.Equal(t, []string{"https://github.com/example/repo"}, comp.Repos)

	// DataInjections should be preserved via the private shim.
	require.Len(t, comp.GetDataInjections(), 1)
	assert.Equal(t, "/data", comp.GetDataInjections()[0].Source)
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
		{"zarf-registry", "zarf-registry", true, false, false},
		{"zarf-injector", "zarf-injector", true, false, false},
		{"zarf-seed-registry", "zarf-seed-registry", true, false, true},
		{"zarf-agent", "zarf-agent", false, true, false},
		{"regular component", "my-app", false, false, false},
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
			assert.Equal(t, tt.isRegistry, comp.Features.IsRegistry, "IsRegistry")
			assert.Equal(t, tt.isAgent, comp.Features.IsAgent, "IsAgent")
			if tt.injector {
				require.NotNil(t, comp.Features.Injector)
				assert.True(t, comp.Features.Injector.Enabled)
			} else {
				assert.Nil(t, comp.Features.Injector)
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
				assert.Equal(t, "https://stefanprodan.github.io/podinfo", c.HelmRepo.URL)
				assert.Equal(t, "podinfo", c.HelmRepo.Name)
				assert.Equal(t, "6.4.0", c.HelmRepo.Version)
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
				assert.Equal(t, "oci://ghcr.io/stefanprodan/charts/podinfo", c.OCI.URL)
				assert.Equal(t, "6.4.0", c.OCI.Version)
			},
		},
		{
			name: "git repo",
			chart: v1alpha1.ZarfChart{
				Name:    "my-chart",
				URL:     "https://github.com/example/repo",
				GitPath: "charts/my-chart",
			},
			validate: func(t *testing.T, c v1beta1.ZarfChart) {
				assert.Equal(t, "https://github.com/example/repo", c.Git.URL)
				assert.Equal(t, "charts/my-chart", c.Git.Path)
			},
		},
		{
			name: "local path",
			chart: v1alpha1.ZarfChart{
				Name:      "local-chart",
				LocalPath: "./charts/my-chart",
			},
			validate: func(t *testing.T, c v1beta1.ZarfChart) {
				assert.Equal(t, "./charts/my-chart", c.Local.Path)
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
	assert.False(t, *m0.Wait)

	// NoWait=false (default) → Wait=nil (v1beta1 defaults to true)
	m1 := result.Components[0].Manifests[1]
	assert.Nil(t, m1.Wait)
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
	assert.True(t, actions.OnDeploy.Defaults.Mute)
	require.NotNil(t, actions.OnDeploy.Defaults.Timeout)
	assert.Equal(t, 60*time.Second, actions.OnDeploy.Defaults.Timeout.Duration)
	assert.Equal(t, 2, actions.OnDeploy.Defaults.Retries)
	assert.Equal(t, "/work", actions.OnDeploy.Defaults.Dir)
	assert.Equal(t, []string{"FOO=bar"}, actions.OnDeploy.Defaults.Env)
	assert.Equal(t, "bash", actions.OnDeploy.Defaults.Shell.Linux)

	// Before action
	require.Len(t, actions.OnDeploy.Before, 1)
	before := actions.OnDeploy.Before[0]
	assert.Equal(t, "echo before", before.Cmd)
	require.NotNil(t, before.Mute)
	assert.True(t, *before.Mute)
	require.NotNil(t, before.Timeout)
	assert.Equal(t, 30*time.Second, before.Timeout.Duration)
	assert.Equal(t, 3, before.Retries)
	assert.Equal(t, "run before", before.Description)
	// SetVariables should include both the explicit one and the deprecated one.
	require.Len(t, before.SetVariables, 2)
	assert.Equal(t, "OUT_VAR", before.SetVariables[0].Name)
	assert.True(t, before.SetVariables[0].Sensitive)
	assert.Equal(t, "OLD_VAR", before.SetVariables[1].Name)

	// After should include original After + OnSuccess (merged).
	require.Len(t, actions.OnDeploy.After, 2)
	assert.Equal(t, "echo after", actions.OnDeploy.After[0].Cmd)
	assert.Equal(t, "echo success", actions.OnDeploy.After[1].Cmd)

	// OnFailure
	require.Len(t, actions.OnDeploy.OnFailure, 1)
	assert.Equal(t, "echo failure", actions.OnDeploy.OnFailure[0].Cmd)
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
	assert.Equal(t, "https://example.com/file.tar.gz", f.Source)
	assert.Equal(t, "deadbeef", f.Shasum)
	assert.Equal(t, "/opt/file.tar.gz", f.Target)
	assert.True(t, f.Executable)
	assert.Equal(t, []string{"/usr/local/bin/file"}, f.Symlinks)
	assert.Equal(t, "bin/file", f.ExtractPath)
	require.NotNil(t, f.Template)
	assert.True(t, *f.Template)
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

	assert.Equal(t, []string{"values.yaml"}, result.Values.Files)
	assert.Equal(t, "values.schema.json", result.Values.Schema)
	assert.Equal(t, "# Hello", result.Documentation["readme"])
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
	assert.Equal(t, "1.2.3", chart.GetDeprecatedVersion())
}

// --- v1beta1 → v1alpha1 tests ---

func TestV1Beta1PkgToV1Alpha1_Metadata(t *testing.T) {
	t.Parallel()
	pkg := v1beta1.ZarfPackage{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata: v1beta1.ZarfMetadata{
			Name:                   "test-pkg",
			Description:            "A test package",
			Version:                "1.0.0",
			Architecture:           "amd64",
			Uncompressed:           true,
			AllowNamespaceOverride: true,
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

	assert.Equal(t, v1alpha1.APIVersion, result.APIVersion)
	assert.Equal(t, v1alpha1.ZarfPackageConfig, result.Kind)
	assert.Equal(t, "test-pkg", result.Metadata.Name)
	assert.Equal(t, "A test package", result.Metadata.Description)
	assert.Equal(t, "1.0.0", result.Metadata.Version)
	assert.Equal(t, "amd64", result.Metadata.Architecture)
	assert.True(t, result.Metadata.Uncompressed)
	require.NotNil(t, result.Metadata.AllowNamespaceOverride)
	assert.True(t, *result.Metadata.AllowNamespaceOverride)

	// v1alpha1-only metadata fields should be restored from annotations.
	assert.Equal(t, "https://example.com", result.Metadata.URL)
	assert.Equal(t, "https://example.com/image.png", result.Metadata.Image)
	assert.Equal(t, "Test Author", result.Metadata.Authors)
	assert.Equal(t, "https://docs.example.com", result.Metadata.Documentation)
	assert.Equal(t, "https://github.com/example", result.Metadata.Source)
	assert.Equal(t, "Example Corp", result.Metadata.Vendor)

	// Metadata-specific annotations should be consumed, regular annotations preserved.
	assert.Equal(t, "annotation", result.Metadata.Annotations["existing"])
	assert.Empty(t, result.Metadata.Annotations["metadata.url"])

	// AggregateChecksum should move from build to metadata.
	assert.Equal(t, "abc123", result.Metadata.AggregateChecksum)
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
			VersionRequirements: []v1beta1.VersionRequirement{
				{Version: "v0.28.0", Reason: "needs feature X"},
			},
			ProvenanceFiles: []string{"sig.json"},
		},
	}

	result := V1Beta1PkgToV1Alpha1(pkg)

	assert.Equal(t, v1alpha1.ZarfInitConfig, result.Kind)
	assert.Equal(t, "my-machine", result.Build.Terminal)
	assert.Equal(t, "test-user", result.Build.User)
	assert.Equal(t, "arm64", result.Build.Architecture)
	assert.Equal(t, "v0.30.0", result.Build.Version)
	assert.True(t, result.Build.Differential)
	assert.Equal(t, "0.29.0", result.Build.DifferentialPackageVersion)
	assert.Equal(t, "vanilla", result.Build.Flavor)
	require.NotNil(t, result.Build.Signed)
	assert.True(t, *result.Build.Signed)
	require.Len(t, result.Build.VersionRequirements, 1)
	assert.Equal(t, "v0.28.0", result.Build.VersionRequirements[0].Version)
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
	assert.Equal(t, "my-component", comp.Name)
	assert.Equal(t, "test component", comp.Description)

	// Optional=true → Required=false
	require.NotNil(t, comp.Required)
	assert.False(t, *comp.Required)

	assert.Equal(t, "linux", comp.Only.LocalOS)
	assert.Equal(t, "amd64", comp.Only.Cluster.Architecture)
	assert.Equal(t, []string{"k3s"}, comp.Only.Cluster.Distros)
	assert.Equal(t, "vanilla", comp.Only.Flavor)

	assert.Equal(t, "./path", comp.Import.Path)
	assert.Equal(t, "oci://example.com/pkg", comp.Import.URL)

	// Images are converted from []ZarfImage to []string.
	require.Len(t, comp.Images, 2)
	assert.Equal(t, "nginx:latest", comp.Images[0])
	assert.Equal(t, "redis:7", comp.Images[1])

	assert.Equal(t, []string{"https://github.com/example/repo"}, comp.Repos)
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
				assert.Equal(t, "https://stefanprodan.github.io/podinfo", c.URL)
				assert.Equal(t, "podinfo", c.RepoName)
				assert.Equal(t, "6.4.0", c.Version)
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
				assert.Equal(t, "oci://ghcr.io/stefanprodan/charts/podinfo", c.URL)
				assert.Equal(t, "6.4.0", c.Version)
			},
		},
		{
			name: "git repo",
			chart: v1beta1.ZarfChart{
				Name: "my-chart",
				Git: v1beta1.GitRepoSource{
					URL:  "https://github.com/example/repo",
					Path: "charts/my-chart",
				},
			},
			validate: func(t *testing.T, c v1alpha1.ZarfChart) {
				assert.Equal(t, "https://github.com/example/repo", c.URL)
				assert.Equal(t, "charts/my-chart", c.GitPath)
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
				assert.Equal(t, "./charts/my-chart", c.LocalPath)
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
	assert.True(t, result.Components[0].Manifests[0].NoWait)

	// Wait=nil → NoWait=false (default)
	assert.False(t, result.Components[0].Manifests[1].NoWait)
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
	assert.True(t, actions.OnDeploy.Defaults.Mute)
	assert.Equal(t, 60, actions.OnDeploy.Defaults.MaxTotalSeconds)
	assert.Equal(t, 2, actions.OnDeploy.Defaults.MaxRetries)
	assert.Equal(t, "/work", actions.OnDeploy.Defaults.Dir)
	assert.Equal(t, []string{"FOO=bar"}, actions.OnDeploy.Defaults.Env)
	assert.Equal(t, "bash", actions.OnDeploy.Defaults.Shell.Linux)

	// Before action
	require.Len(t, actions.OnDeploy.Before, 1)
	before := actions.OnDeploy.Before[0]
	assert.Equal(t, "echo before", before.Cmd)
	require.NotNil(t, before.Mute)
	assert.True(t, *before.Mute)
	require.NotNil(t, before.MaxTotalSeconds)
	assert.Equal(t, 30, *before.MaxTotalSeconds)
	require.NotNil(t, before.MaxRetries)
	assert.Equal(t, 3, *before.MaxRetries)
	require.Len(t, before.SetVariables, 1)
	assert.Equal(t, "OUT_VAR", before.SetVariables[0].Name)
	assert.True(t, before.SetVariables[0].Sensitive)

	// After
	require.Len(t, actions.OnDeploy.After, 1)
	assert.Equal(t, "echo after", actions.OnDeploy.After[0].Cmd)

	// OnFailure
	require.Len(t, actions.OnDeploy.OnFailure, 1)
	assert.Equal(t, "echo failure", actions.OnDeploy.OnFailure[0].Cmd)
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
	assert.Equal(t, "MY_VAR", v.Name)
	assert.True(t, v.Sensitive)
	assert.True(t, v.AutoIndent)
	assert.Equal(t, "^[a-z]+$", v.Pattern)
	assert.Equal(t, v1alpha1.FileVariableType, v.Type)
	assert.Equal(t, "A variable", v.Description)
	assert.Equal(t, "default-val", v.Default)
	assert.True(t, v.Prompt)

	require.Len(t, result.Constants, 1)
	c := result.Constants[0]
	assert.Equal(t, "MY_CONST", c.Name)
	assert.Equal(t, "const-val", c.Value)
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

	assert.Equal(t, v1alpha1.APIVersion, result.APIVersion)
	assert.Equal(t, original.Kind, result.Kind)
	assert.Equal(t, original.Metadata.Name, result.Metadata.Name)
	assert.Equal(t, original.Metadata.Description, result.Metadata.Description)
	assert.Equal(t, original.Metadata.Version, result.Metadata.Version)
	assert.Equal(t, original.Metadata.Architecture, result.Metadata.Architecture)
	assert.Equal(t, original.Metadata.URL, result.Metadata.URL)
	assert.Equal(t, original.Metadata.Authors, result.Metadata.Authors)

	require.Len(t, result.Components, 1)
	comp := result.Components[0]
	assert.Equal(t, "test-comp", comp.Name)
	require.NotNil(t, comp.Required)
	assert.True(t, *comp.Required)
	assert.Equal(t, []string{"nginx:latest"}, comp.Images)

	// Chart should round-trip via structured source → flat fields.
	require.Len(t, comp.Charts, 1)
	assert.Equal(t, "https://charts.example.com", comp.Charts[0].URL)
	assert.Equal(t, "my-chart", comp.Charts[0].RepoName)
	assert.Equal(t, "1.2.3", comp.Charts[0].Version)

	// Manifest NoWait should survive round-trip.
	require.Len(t, comp.Manifests, 1)
	assert.True(t, comp.Manifests[0].NoWait)

	// Action timeouts should round-trip through Duration conversion.
	assert.Equal(t, 60, comp.Actions.OnDeploy.Defaults.MaxTotalSeconds)
	assert.Equal(t, 2, comp.Actions.OnDeploy.Defaults.MaxRetries)
	require.Len(t, comp.Actions.OnDeploy.Before, 1)
	require.NotNil(t, comp.Actions.OnDeploy.Before[0].MaxTotalSeconds)
	assert.Equal(t, 30, *comp.Actions.OnDeploy.Before[0].MaxTotalSeconds)

	// DataInjections should survive round-trip via private shim.
	require.Len(t, comp.DataInjections, 1)
	assert.Equal(t, "/data", comp.DataInjections[0].Source)

	// Constants and variables.
	require.Len(t, result.Constants, 1)
	assert.Equal(t, "MY_CONST", result.Constants[0].Name)
	require.Len(t, result.Variables, 1)
	assert.Equal(t, "MY_VAR", result.Variables[0].Name)
}
