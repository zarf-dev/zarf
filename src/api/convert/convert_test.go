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
