// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package assemble

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestApplyDifferentialResourcesV1alpha1(t *testing.T) {
	t.Parallel()

	current := api.NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{
			{
				Images: []string{
					"example.com/include-image-tag:latest",
					"example.com/image-with-tag:v1",
					"example.com/diff-image-with-tag:v1",
					"example.com/image-with-digest@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					"example.com/diff-image-with-digest@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					"example.com/image-with-tag-and-digest:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					"example.com/diff-image-with-tag-and-digest:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
				Repos: []string{
					"https://example.com/no-ref.git",
					"https://example.com/branch.git@refs/heads/main",
					"https://example.com/tag.git@v1",
					"https://example.com/diff-tag.git@v1",
					"https://example.com/commit.git@524980951ff16e19dc25232e9aea8fd693989ba6",
					"https://example.com/diff-commit.git@524980951ff16e19dc25232e9aea8fd693989ba6",
				},
			},
		},
	})
	previous := api.NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{
			{
				Images: []string{
					"example.com/include-image-tag:latest",
					"example.com/diff-image-with-tag:v1",
					"example.com/diff-image-with-digest@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					"example.com/diff-image-with-tag-and-digest:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				},
				Repos: []string{
					"https://example.com/no-ref.git",
					"https://example.com/branch.git@refs/heads/main",
					"https://example.com/diff-tag.git@v1",
					"https://example.com/diff-commit.git@524980951ff16e19dc25232e9aea8fd693989ba6",
				},
			},
		},
	})

	result, err := applyDifferentialResources(current, previous)
	require.NoError(t, err)

	pkg := result.AsV1alpha1()
	require.ElementsMatch(t, []string{
		"example.com/include-image-tag:latest",
		"example.com/image-with-tag:v1",
		"example.com/image-with-digest@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"example.com/image-with-tag-and-digest:v1@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}, pkg.Components[0].Images)
	require.ElementsMatch(t, []string{
		"https://example.com/no-ref.git",
		"https://example.com/branch.git@refs/heads/main",
		"https://example.com/tag.git@v1",
		"https://example.com/commit.git@524980951ff16e19dc25232e9aea8fd693989ba6",
	}, pkg.Components[0].Repos)
}

func TestApplyDifferentialResourcesV1beta1PreservesResourceFields(t *testing.T) {
	t.Parallel()

	current := api.NewPackageDefinitionFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Components: []v1beta1.Component{
			{
				Name: "component",
				ComponentSpec: v1beta1.ComponentSpec{
					Images: []v1beta1.Image{
						{Name: "registry.example.com/mutable:latest", Source: "daemon"},
						{Name: "registry.example.com/new:v1", Source: "daemon"},
						{Name: "registry.example.com/removed:v1", Source: "registry"},
					},
					Repositories: []v1beta1.Repository{
						{URL: "https://example.com/no-ref.git"},
						{URL: "https://example.com/branch.git", Ref: &v1beta1.GitRef{Branch: "main"}},
						{URL: "https://example.com/new-tag.git", Ref: &v1beta1.GitRef{Tag: "v1"}},
						{URL: "https://example.com/removed-tag.git", Ref: &v1beta1.GitRef{Tag: "v1"}},
					},
				},
			},
		},
	})
	previous := api.NewPackageDefinitionFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Components: []v1beta1.Component{
			{
				Name: "component",
				ComponentSpec: v1beta1.ComponentSpec{
					Images: []v1beta1.Image{
						{Name: "registry.example.com/mutable:latest", Source: "daemon"},
						{Name: "registry.example.com/removed:v1", Source: "registry"},
					},
					Repositories: []v1beta1.Repository{
						{URL: "https://example.com/no-ref.git"},
						{URL: "https://example.com/branch.git", Ref: &v1beta1.GitRef{Branch: "main"}},
						{URL: "https://example.com/removed-tag.git", Ref: &v1beta1.GitRef{Tag: "v1"}},
					},
				},
			},
		},
	})

	result, err := applyDifferentialResources(current, previous)
	require.NoError(t, err)

	pkg := result.AsV1beta1()
	require.Equal(t, []v1beta1.Image{
		{Name: "registry.example.com/mutable:latest", Source: "daemon"},
		{Name: "registry.example.com/new:v1", Source: "daemon"},
	}, pkg.Components[0].Images)
	require.Equal(t, []v1beta1.Repository{
		{URL: "https://example.com/no-ref.git"},
		{URL: "https://example.com/branch.git", Ref: &v1beta1.GitRef{Branch: "main"}},
		{URL: "https://example.com/new-tag.git", Ref: &v1beta1.GitRef{Tag: "v1"}},
	}, pkg.Components[0].Repositories)
}

func TestApplyDifferentialResourcesRequiresOriginalAPIVersion(t *testing.T) {
	t.Parallel()

	unsupported := v1alpha1.ZarfPackage{}
	unsupported.Build.SetOriginalAPIVersion("zarf.dev/v0")

	tests := []struct {
		name       string
		definition api.PackageDefinition
	}{
		{
			name:       "missing original api version",
			definition: api.PackageDefinition{},
		},
		{
			name:       "unsupported original api version",
			definition: api.NewPackageDefinitionFromV1alpha1(unsupported),
		},
	}
	previous := api.NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := applyDifferentialResources(tt.definition, previous)

			require.Error(t, err)
		})
	}
}

func TestAssemblePackageDifferentialRequiresSameAPIVersion(t *testing.T) {
	t.Parallel()

	ctx := testutil.TestContext(t)
	current := api.NewPackageDefinitionFromV1beta1(v1beta1.Package{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageConfig,
		Metadata: v1beta1.PackageMetadata{
			Name:    "differential-test",
			Version: "0.0.2",
		},
	})
	previous := v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name:    "differential-test",
			Version: "0.0.1",
		},
	}

	_, err := AssemblePackage(ctx, load.ResolvedPackage{PackageDefinition: current}, t.TempDir(), AssembleOptions{
		DifferentialPackage: previous,
		SkipSBOM:            true,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), lang.PkgCreateErrDifferentialAPIVersion)
	require.Contains(t, err.Error(), "package apiVersion "+v1beta1.APIVersion)
	require.Contains(t, err.Error(), "differential package apiVersion "+v1alpha1.APIVersion)
}
