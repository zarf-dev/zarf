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

func TestPackageDefinitionOverrideNamespace(t *testing.T) {
	t.Parallel()

	falsePtr := func() *bool { value := false; return &value }
	newDefinition := func(pkg v1alpha1.ZarfPackage) PackageDefinition {
		return NewPackageDefinitionFromV1alpha1(pkg)
	}
	tests := []struct {
		name        string
		definition  PackageDefinition
		namespace   string
		expectedErr string
		verify      func(*testing.T, v1alpha1.ZarfPackage)
	}{
		{
			name: "updates charts manifests and matching wait actions",
			definition: newDefinition(v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{{
					Charts:    []v1alpha1.ZarfChart{{Name: "chart", Namespace: "original"}},
					Manifests: []v1alpha1.ZarfManifest{{Name: "manifest", Namespace: "original"}},
					Actions: v1alpha1.ZarfComponentActions{OnDeploy: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{namespaceWaitAction("original"), namespaceWaitAction("other")},
					}},
				}},
			}),
			namespace: "target",
			verify: func(t *testing.T, pkg v1alpha1.ZarfPackage) {
				t.Helper()
				require.Equal(t, "target", pkg.Components[0].Charts[0].Namespace)
				require.Equal(t, "target", pkg.Components[0].Manifests[0].Namespace)
				require.Equal(t, []string{"target", "other"}, namespaceWaitNamespaces(pkg.Components[0].Actions))
			},
		},
		{
			name: "updates every action lifecycle and timing slot",
			definition: newDefinition(v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{{
					Charts:  []v1alpha1.ZarfChart{{Name: "chart", Namespace: "original"}},
					Actions: allNamespaceWaitActions("original"),
				}},
			}),
			namespace: "target",
			verify: func(t *testing.T, pkg v1alpha1.ZarfPackage) {
				t.Helper()
				require.ElementsMatch(t, []string{"target", "target", "target", "target", "target", "target", "target", "target", "target", "target", "target", "target"}, namespaceWaitNamespaces(pkg.Components[0].Actions))
			},
		},
		{
			name: "updates an empty namespace",
			definition: newDefinition(v1alpha1.ZarfPackage{
				Kind:       v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{{Charts: []v1alpha1.ZarfChart{{Name: "chart"}}}},
			}),
			namespace: "target",
			verify: func(t *testing.T, pkg v1alpha1.ZarfPackage) {
				t.Helper()
				require.Equal(t, "target", pkg.Components[0].Charts[0].Namespace)
			},
		},
		{
			name: "handles nil wait actions",
			definition: newDefinition(v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{{
					Charts: []v1alpha1.ZarfChart{{Name: "chart", Namespace: "original"}},
					Actions: v1alpha1.ZarfComponentActions{OnDeploy: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{{}, {Wait: &v1alpha1.ZarfComponentActionWait{}}},
					}},
				}},
			}),
			namespace: "target",
			verify: func(t *testing.T, pkg v1alpha1.ZarfPackage) {
				t.Helper()
				require.Equal(t, "target", pkg.Components[0].Charts[0].Namespace)
			},
		},
		{
			name: "rejects multiple namespaces",
			definition: newDefinition(v1alpha1.ZarfPackage{
				Kind:       v1alpha1.ZarfPackageConfig,
				Components: []v1alpha1.ZarfComponent{{Charts: []v1alpha1.ZarfChart{{Name: "one", Namespace: "one"}, {Name: "two", Namespace: "two"}}}},
			}),
			namespace:   "target",
			expectedErr: "package contains 2 unique namespaces, cannot override namespace",
		},
		{
			name: "rejects init packages",
			definition: newDefinition(v1alpha1.ZarfPackage{
				Kind: v1alpha1.ZarfInitConfig,
			}),
			namespace:   "target",
			expectedErr: "package kind is not a ZarfPackageConfig, cannot override namespace",
		},
		{
			name: "honors prevent namespace override",
			definition: newDefinition(v1alpha1.ZarfPackage{
				Kind:     v1alpha1.ZarfPackageConfig,
				Metadata: v1alpha1.ZarfMetadata{AllowNamespaceOverride: falsePtr()},
			}),
			namespace:   "target",
			expectedErr: "package explicitly prevents namespace overrides",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.definition.OverrideNamespace(tt.namespace)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			tt.verify(t, tt.definition.AsV1alpha1())
		})
	}
}

func namespaceWaitAction(namespace string) v1alpha1.ZarfComponentAction {
	return v1alpha1.ZarfComponentAction{
		Wait: &v1alpha1.ZarfComponentActionWait{
			Cluster: &v1alpha1.ZarfComponentActionWaitCluster{Namespace: namespace},
		},
	}
}

func allNamespaceWaitActions(namespace string) v1alpha1.ZarfComponentActions {
	newActions := func() []v1alpha1.ZarfComponentAction {
		return []v1alpha1.ZarfComponentAction{namespaceWaitAction(namespace)}
	}
	return v1alpha1.ZarfComponentActions{
		OnCreate: v1alpha1.ZarfComponentActionSet{Before: newActions(), After: newActions(), OnSuccess: newActions(), OnFailure: newActions()},
		OnDeploy: v1alpha1.ZarfComponentActionSet{Before: newActions(), After: newActions(), OnSuccess: newActions(), OnFailure: newActions()},
		OnRemove: v1alpha1.ZarfComponentActionSet{Before: newActions(), After: newActions(), OnSuccess: newActions(), OnFailure: newActions()},
	}
}

func namespaceWaitNamespaces(actions v1alpha1.ZarfComponentActions) []string {
	var namespaces []string
	for _, actionSet := range [][]v1alpha1.ZarfComponentAction{
		actions.OnCreate.Before, actions.OnCreate.After, actions.OnCreate.OnSuccess, actions.OnCreate.OnFailure,
		actions.OnDeploy.Before, actions.OnDeploy.After, actions.OnDeploy.OnSuccess, actions.OnDeploy.OnFailure,
		actions.OnRemove.Before, actions.OnRemove.After, actions.OnRemove.OnSuccess, actions.OnRemove.OnFailure,
	} {
		for _, action := range actionSet {
			if action.Wait != nil && action.Wait.Cluster != nil {
				namespaces = append(namespaces, action.Wait.Cluster.Namespace)
			}
		}
	}
	return namespaces
}
