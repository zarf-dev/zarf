// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

// TestConvertGenericRoundTripLossless asserts that a v1beta1 package converted to the generic
// representation and back reproduces the original exactly. layout and zoci load built packages
// through this round-trip, so any drift would change packages across build hosts.
func TestConvertGenericRoundTripLossless(t *testing.T) {
	t.Parallel()

	b := func(v bool) *bool { return &v }
	i := func(v int32) *int32 { return &v }
	s := func(v string) *string { return &v }

	original := v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata: v1beta1.PackageMetadata{
			Name:                     "round-trip",
			Description:              "desc",
			Version:                  "1.2.3",
			Uncompressed:             true,
			Architecture:             "arm64",
			Annotations:              map[string]string{"k": "v"},
			PreventNamespaceOverride: true,
		},
		Build: v1beta1.BuildData{
			Hostname:                   "host",
			User:                       "user",
			Architecture:               "arm64",
			Timestamp:                  "Mon, 02 Jan 2006 15:04:05 -0700",
			Version:                    "v0.30.0",
			Migrations:                 []string{"scripts-to-actions", "pluralize-set-variable"},
			RegistryOverrides:          map[string]string{"reg": "override"},
			Differential:               true,
			DifferentialPackageVersion: "1.2.2",
			Flavor:                     "prod",
			Signed:                     b(true),
			VersionRequirements:        []v1beta1.VersionRequirement{{Version: ">=1.0.0", Reason: "needs feature"}},
			ProvenanceFiles:            []string{"prov.json"},
			AggregateChecksum:          "checksum",
		},
		Components: []v1beta1.Component{
			{
				Name:        "comp",
				Description: "comp desc",
				Optional:    true,
				ComponentSpec: v1beta1.ComponentSpec{
					Import: v1beta1.ComponentImport{
						Local:  []v1beta1.ComponentImportLocal{{Path: "path"}},
						Remote: []v1beta1.ComponentImportRemote{{URL: "oci://example.com/pkg"}},
					},
					Target: v1beta1.ComponentTarget{
						OS:           "linux",
						Architecture: "arm64",
						Flavor:       "prod",
					},
					Service:      v1beta1.ServiceRegistry,
					Repositories: []v1beta1.Repository{{URL: "https://github.com/example/repo"}},
					StateAccess:  []v1beta1.StateAccessKey{v1beta1.StateAccessRegistryCredentials},
					Images: []v1beta1.Image{
						{Name: "nginx:latest", Source: "registry"},
					},
					ImageArchives: []v1beta1.ImageArchive{
						{Path: "images.tar", Images: []string{"busybox:1.36"}},
					},
					Charts: []v1beta1.Chart{
						{
							Name:                 "helm-chart",
							Namespace:            "default",
							ReleaseName:          "rel",
							SkipWait:             true,
							ValuesFiles:          []string{"values.yaml"},
							SkipSchemaValidation: true,
							ServerSideApply:      v1beta1.ServerSideApplyAuto,
							HelmRepository:       &v1beta1.HelmRepositorySource{Name: "chart", URL: "https://charts.example.com", Version: "1.0.0"},
							Values:               []v1beta1.ChartValue{{SourcePath: ".a", TargetPath: ".b"}},
						},
						{
							Name: "git-chart",
							Git:  &v1beta1.GitSource{URL: "https://github.com/example/repo.git", Path: "charts/app"},
						},
						{
							Name:  "local-chart",
							Local: &v1beta1.LocalSource{Path: "chart"},
						},
						{
							Name: "oci-chart",
							OCI:  &v1beta1.OCISource{URL: "oci://example.com/chart", Version: "2.0.0"},
						},
					},
					Manifests: []v1beta1.Manifest{
						{
							Name:      "manifest",
							Namespace: "default",
							Files:     []string{"deploy.yaml"},
							Kustomize: &v1beta1.KustomizeManifest{
								Files:             []string{"kustomize"},
								AllowAnyDirectory: true,
								EnablePlugins:     true,
							},
							SkipWait:         true,
							ServerSideApply:  v1beta1.ServerSideApplyEnabled,
							EnableTemplating: true,
						},
					},
					Files: []v1beta1.File{
						{Source: "src", Checksum: "sha", Destination: "tgt", Executable: true, Symlinks: []string{"ln"}, ExtractPath: "extract", EnableTemplating: true},
					},
					Actions: v1beta1.ComponentActions{
						OnDeploy: v1beta1.ComponentActionSet{
							Defaults: v1beta1.ComponentActionDefaults{
								Silent:          true,
								MaxTotalSeconds: 60,
								Retries:         2,
								Dir:             "dir",
								Env:             []string{"K=V"},
								Shell:           v1beta1.Shell{Windows: "pwsh", Linux: "sh", Darwin: "zsh"},
							},
							Before: []v1beta1.ComponentAction{
								{
									Silent:           b(true),
									MaxTotalSeconds:  i(30),
									Retries:          i(3),
									Dir:              s("d"),
									Env:              []string{"A=B"},
									Cmd:              "before",
									Shell:            &v1beta1.Shell{Windows: "pwsh", Linux: "sh", Darwin: "zsh"},
									SetValues:        []v1beta1.SetValue{{Key: "key", Type: v1beta1.SetValueYAML}},
									Description:      "before action",
									EnableTemplating: true,
								},
							},
							OnSuccess: []v1beta1.ComponentAction{
								{
									Cmd: "success",
									Wait: &v1beta1.ComponentActionWait{
										Cluster: &v1beta1.ComponentActionWaitCluster{Kind: "Pod", Name: "n", Namespace: "ns", Condition: "Ready"},
									},
								},
							},
							OnFailure: []v1beta1.ComponentAction{
								{
									Cmd: "failure",
									Wait: &v1beta1.ComponentActionWait{
										Network: &v1beta1.ComponentActionWaitNetwork{Protocol: "http", Address: "localhost:8080", Code: 200},
									},
								},
							},
						},
					},
				},
			},
		},
		Values:        v1beta1.Values{Files: []string{"vals.yaml"}, Schema: "schema.json"},
		Documentation: map[string]string{"doc": "doc.md"},
	}

	roundTripped := ConvertFromGeneric(ConvertToGeneric(original))
	require.Equal(t, original, roundTripped)
}
