// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

func TestPackageDefinitionSetComponentResources(t *testing.T) {
	t.Parallel()

	definition := NewPackageDefinitionFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Components: []v1beta1.Component{
			{
				Name: "component",
				ComponentSpec: v1beta1.ComponentSpec{
					Images: []v1beta1.Image{
						{Name: "registry.example.com/kept:v1", Source: "daemon"},
						{Name: "registry.example.com/removed:v1", Source: "registry"},
					},
					Repositories: []v1beta1.Repository{{URL: "https://example.com/old.git"}},
				},
			},
		},
	})

	definition.SetComponentImages("component", []string{"registry.example.com/kept:v1", "registry.example.com/new:v1"})
	definition.SetComponentRepositories("component", []string{"https://example.com/new.git@v1"})

	beta := definition.AsV1beta1()
	require.Len(t, beta.Components, 1)
	require.Equal(t, []v1beta1.Image{
		{Name: "registry.example.com/kept:v1", Source: "daemon"},
		{Name: "registry.example.com/new:v1"},
	}, beta.Components[0].Images)
	require.Len(t, beta.Components[0].Repositories, 1)
	require.Equal(t, "https://example.com/new.git", beta.Components[0].Repositories[0].URL)
	require.NotNil(t, beta.Components[0].Repositories[0].Ref)
	require.Equal(t, "v1", beta.Components[0].Repositories[0].Ref.Tag)

	alpha := definition.AsV1alpha1()
	require.Equal(t, []string{"registry.example.com/kept:v1", "registry.example.com/new:v1"}, alpha.Components[0].Images)
	require.Equal(t, []string{"https://example.com/new.git@v1"}, alpha.Components[0].Repos)
}
