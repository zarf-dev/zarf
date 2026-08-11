// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestPackageDefinitionSetChartNamespace(t *testing.T) {
	t.Parallel()

	definition := NewPackageDefinitionFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Components: []v1beta1.Component{
			{
				Name: "component",
				ComponentSpec: v1beta1.ComponentSpec{
					Charts: []v1beta1.Chart{
						{
							Name:      "app",
							Namespace: "old",
							OCI: &v1beta1.OCISource{
								URL: "oci://example.com/app",
								Ref: v1beta1.OCIRef{Tag: "1.0.0"},
							},
						},
						{Name: "unchanged", Namespace: "old"},
					},
				},
			},
		},
	})

	definition.SetChartNamespace("component", "app", "new")

	beta := definition.AsV1beta1()
	require.Equal(t, "new", beta.Components[0].Charts[0].Namespace)
	require.Equal(t, "oci://example.com/app", beta.Components[0].Charts[0].OCI.URL)
	require.Equal(t, "old", beta.Components[0].Charts[1].Namespace)

	alpha := definition.AsV1alpha1()
	require.Equal(t, "new", alpha.Components[0].Charts[0].Namespace)
	require.Equal(t, "old", alpha.Components[0].Charts[1].Namespace)
}

func TestPackageDefinitionRemoveImagesAndRepositories(t *testing.T) {
	t.Parallel()

	definition := NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name: "package",
		},
		Components: []v1alpha1.ZarfComponent{
			{
				Name:          "component",
				Images:        []string{"registry.example.com/app:v1"},
				ImageArchives: []v1alpha1.ImageArchive{{Path: "images.tar", Images: []string{"registry.example.com/archive:v1"}}},
				Repos:         []string{"https://example.com/repo.git"},
			},
		},
	})

	definition.RemoveImages()

	alpha := definition.AsV1alpha1()
	require.Empty(t, alpha.Components[0].Images)
	require.Empty(t, alpha.Components[0].ImageArchives)
	require.Equal(t, []string{"https://example.com/repo.git"}, alpha.Components[0].Repos)

	beta := definition.AsV1beta1()
	require.Empty(t, beta.Components[0].Images)
	require.Empty(t, beta.Components[0].ImageArchives)
	require.Len(t, beta.Components[0].Repositories, 1)

	definition.RemoveRepositories()

	alpha = definition.AsV1alpha1()
	require.Empty(t, alpha.Components[0].Repos)

	beta = definition.AsV1beta1()
	require.Empty(t, beta.Components[0].Repositories)
}

func TestPackageDefinitionRetainComponents(t *testing.T) {
	t.Parallel()

	definition := NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{
			{Name: "first"},
			{Name: "second"},
			{Name: "third"},
		},
	})

	err := definition.RetainComponents([]int{2, 0})

	require.NoError(t, err)
	require.Equal(t, []v1alpha1.ZarfComponent{{Name: "third"}, {Name: "first"}}, definition.AsV1alpha1().Components)
}

func TestPackageDefinitionRetainComponents_invalidIndexDoesNotModifyDefinition(t *testing.T) {
	t.Parallel()

	definition := NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{{Name: "first"}, {Name: "second"}},
	})

	err := definition.RetainComponents([]int{0, 2})

	require.EqualError(t, err, "component index 2 out of range")
	require.Equal(t, []v1alpha1.ZarfComponent{{Name: "first"}, {Name: "second"}}, definition.AsV1alpha1().Components)
}

func TestPackageDefinitionSetBuildData(t *testing.T) {
	t.Parallel()

	definition := NewPackageDefinitionFromV1beta1(v1beta1.Package{})
	signed := false
	buildData := BuildData{
		Hostname:            "build-host",
		User:                "builder",
		Architecture:        "amd64",
		Timestamp:           "timestamp",
		Version:             "v1.0.0",
		RegistryOverrides:   map[string]string{"registry.example.com": "registry.internal"},
		Flavor:              "fips",
		Signed:              &signed,
		VersionRequirements: []VersionRequirement{{Version: "v1.0.0", Reason: "feature"}},
		ProvenanceFiles:     []string{"checksums.txt"},
		AggregateChecksum:   "checksum",
	}
	definition.SetBuildData(buildData)
	buildData.RegistryOverrides["registry.example.com"] = "changed"
	signed = true
	definition.AddVersionRequirement(VersionRequirement{Version: "v2.0.0", Reason: "another feature"})
	definition.AddVersionRequirement(VersionRequirement{Version: "v1.0.0", Reason: "feature"})

	beta := definition.AsV1beta1()
	require.Equal(t, "build-host", beta.Build.Hostname)
	require.Equal(t, "builder", beta.Build.User)
	require.Equal(t, "amd64", beta.Build.Architecture)
	require.Equal(t, "timestamp", beta.Build.Timestamp)
	require.Equal(t, "v1.0.0", beta.Build.Version)
	require.Equal(t, map[string]string{"registry.example.com": "registry.internal"}, beta.Build.RegistryOverrides)
	require.Equal(t, "fips", beta.Build.Flavor)
	require.False(t, *beta.Build.Signed)
	require.Equal(t, []string{"checksums.txt"}, beta.Build.ProvenanceFiles)
	require.Equal(t, "checksum", beta.Build.AggregateChecksum)

	require.Equal(t, []v1alpha1.VersionRequirement{
		{Version: "v1.0.0", Reason: "feature"},
		{Version: "v2.0.0", Reason: "another feature"},
	}, definition.AsV1alpha1().Build.VersionRequirements)
	require.Equal(t, "checksum", definition.AsV1alpha1().Metadata.AggregateChecksum)
	require.Equal(t, []v1beta1.VersionRequirement{
		{Version: "v1.0.0", Reason: "feature"},
		{Version: "v2.0.0", Reason: "another feature"},
	}, beta.Build.VersionRequirements)
}

func TestPackageDefinitionSetDifferentialBuild(t *testing.T) {
	t.Parallel()

	definition := NewPackageDefinitionFromV1beta1(v1beta1.Package{})
	definition.SetDifferentialBuild("v1.0.0")

	alpha := definition.AsV1alpha1()
	require.True(t, alpha.Build.Differential)
	require.Equal(t, "v1.0.0", alpha.Build.DifferentialPackageVersion)
	require.Empty(t, alpha.Build.DifferentialMissing)
}

func TestPackageDefinitionNamespaceOverrideConversions(t *testing.T) {
	t.Parallel()

	boolPtr := func(value bool) *bool { return &value }
	tests := []struct {
		name                    string
		definition              PackageDefinition
		allowsNamespaceOverride bool
		explicitAlphaValue      bool
	}{
		{
			name:                    "v1alpha1 default allows namespace override",
			definition:              NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{}),
			allowsNamespaceOverride: true,
		},
		{
			name: "v1alpha1 explicitly prevents namespace override",
			definition: NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{
				Metadata: v1alpha1.ZarfMetadata{AllowNamespaceOverride: boolPtr(false)},
			}),
			allowsNamespaceOverride: false,
			explicitAlphaValue:      true,
		},
		{
			name:                    "v1beta1 allows namespace override",
			definition:              NewPackageDefinitionFromV1beta1(v1beta1.Package{}),
			allowsNamespaceOverride: true,
			explicitAlphaValue:      true,
		},
		{
			name: "v1beta1 prevents namespace override",
			definition: NewPackageDefinitionFromV1beta1(v1beta1.Package{
				Metadata: v1beta1.PackageMetadata{PreventNamespaceOverride: true},
			}),
			allowsNamespaceOverride: false,
			explicitAlphaValue:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alpha := tt.definition.AsV1alpha1()
			beta := tt.definition.AsV1beta1()

			require.Equal(t, tt.allowsNamespaceOverride, alpha.AllowsNamespaceOverride())
			require.Equal(t, !tt.allowsNamespaceOverride, beta.Metadata.PreventNamespaceOverride)
			if tt.explicitAlphaValue {
				require.NotNil(t, alpha.Metadata.AllowNamespaceOverride)
				require.Equal(t, tt.allowsNamespaceOverride, *alpha.Metadata.AllowNamespaceOverride)
			} else {
				require.Nil(t, alpha.Metadata.AllowNamespaceOverride)
			}
		})
	}
}

func TestPackageDefinitionSetMetadata(t *testing.T) {
	t.Parallel()

	definition := NewPackageDefinitionFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata: v1beta1.PackageMetadata{
			Name:        "old",
			Annotations: map[string]string{"old": "value"},
		},
	})

	definition.SetName("new")
	definition.SetAnnotations(map[string]string{"new": "value"})
	definition.SetMetadataArchitecture("arm64")

	beta := definition.AsV1beta1()
	require.Equal(t, "new", beta.Metadata.Name)
	require.Equal(t, map[string]string{"new": "value"}, beta.Metadata.Annotations)
	require.Equal(t, "arm64", beta.Metadata.Architecture)

	alpha := definition.AsV1alpha1()
	require.Equal(t, "new", alpha.Metadata.Name)
	require.Equal(t, map[string]string{"new": "value"}, alpha.Metadata.Annotations)
	require.Equal(t, "arm64", alpha.Metadata.Architecture)
}
