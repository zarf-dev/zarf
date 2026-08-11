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

func TestPackageDefinitionFilterViewIncludesFullComponentDefinition(t *testing.T) {
	t.Parallel()

	alphaComponent := v1alpha1.ZarfComponent{
		Name:        "component",
		Description: "a complete component",
		Images:      []string{"registry.example.com/app:v1"},
		Files: []v1alpha1.ZarfFile{
			{Source: "app.yaml", Target: "/tmp/app.yaml"},
		},
	}
	betaComponent := v1beta1.Component{
		Name:        "component",
		Description: "a complete component",
		ComponentSpec: v1beta1.ComponentSpec{
			Images: []v1beta1.Image{{Name: "registry.example.com/app:v1"}},
			Files:  []v1beta1.File{{Source: "app.yaml", Destination: "/tmp/app.yaml"}},
		},
	}
	tests := []struct {
		name       string
		definition PackageDefinition
		want       any
	}{
		{
			name: "v1alpha1",
			definition: NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{
				Components: []v1alpha1.ZarfComponent{alphaComponent},
			}),
			want: alphaComponent,
		},
		{
			name: "v1beta1",
			definition: NewPackageDefinitionFromV1beta1(v1beta1.Package{
				Components: []v1beta1.Component{betaComponent},
			}),
			want: betaComponent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.definition.filterView()

			require.Len(t, view.Components, 1)
			require.Equal(t, tt.want, view.Components[0].Definition)
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
