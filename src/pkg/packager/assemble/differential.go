// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package assemble

import (
	"fmt"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func applyDifferentialResources(definition, previous api.PackageDefinition) (api.PackageDefinition, error) {
	switch definition.OriginalAPIVersion() {
	case v1beta1.APIVersion:
		pkg := definition.AsV1beta1()
		previousImages, previousRepos := v1beta1DifferentialResources(previous.AsV1beta1().Components)
		for componentIdx, component := range pkg.Components {
			images := make([]v1beta1.Image, 0, len(component.Images))
			for _, img := range component.Images {
				includeImage, err := includeDifferentialImage(img.Name, previousImages)
				if err != nil {
					return api.PackageDefinition{}, err
				}
				if includeImage {
					images = append(images, img)
				}
			}
			pkg.Components[componentIdx].Images = images

			repos := make([]v1beta1.Repository, 0, len(component.Repositories))
			for _, repo := range component.Repositories {
				if includeDifferentialV1beta1Repository(repo, previousRepos) {
					repos = append(repos, repo)
				}
			}
			pkg.Components[componentIdx].Repositories = repos
		}
		return api.NewPackageDefinitionFromV1beta1(pkg), nil
	case v1alpha1.APIVersion:
		pkg := definition.AsV1alpha1()
		previousImages, previousRepos := v1alpha1DifferentialResources(previous.AsV1alpha1().Components)
		for componentIdx, component := range pkg.Components {
			images := make([]string, 0, len(component.Images))
			for _, img := range component.Images {
				includeImage, err := includeDifferentialImage(img, previousImages)
				if err != nil {
					return api.PackageDefinition{}, err
				}
				if includeImage {
					images = append(images, img)
				}
			}
			pkg.Components[componentIdx].Images = images

			repos := make([]string, 0, len(component.Repos))
			for _, repo := range component.Repos {
				includeRepo, err := includeDifferentialRepository(repo, previousRepos)
				if err != nil {
					return api.PackageDefinition{}, err
				}
				if includeRepo {
					repos = append(repos, repo)
				}
			}
			pkg.Components[componentIdx].Repos = repos
		}
		return api.NewPackageDefinitionFromV1alpha1(pkg), nil
	default:
		return api.PackageDefinition{}, fmt.Errorf("unsupported original apiVersion %q", definition.OriginalAPIVersion())
	}
}

func v1alpha1DifferentialResources(components []v1alpha1.ZarfComponent) (map[string]struct{}, map[string]struct{}) {
	images := map[string]struct{}{}
	repos := map[string]struct{}{}
	for _, component := range components {
		for _, image := range component.Images {
			images[image] = struct{}{}
		}
		for _, repo := range component.Repos {
			repos[repo] = struct{}{}
		}
	}
	return images, repos
}

func v1beta1DifferentialResources(components []v1beta1.Component) (map[string]struct{}, []v1beta1.Repository) {
	images := map[string]struct{}{}
	var repos []v1beta1.Repository
	for _, component := range components {
		for _, image := range component.Images {
			images[image.Name] = struct{}{}
		}
		repos = append(repos, component.Repositories...)
	}
	return images, repos
}

func includeDifferentialImage(img string, previousImages map[string]struct{}) (bool, error) {
	imgRef, err := transform.ParseImageRef(img)
	if err != nil {
		return false, fmt.Errorf("unable to parse image ref %s: %w", img, err)
	}
	imgTag := imgRef.TagOrDigest
	includeImage := imgTag == ":latest" || imgTag == ":stable" || imgTag == ":nightly"
	_, inPrevious := previousImages[img]
	return includeImage || !inPrevious, nil
}

func includeDifferentialRepository(repoURL string, previousRepos map[string]struct{}) (bool, error) {
	_, refPlain, err := transform.GitURLSplitRef(repoURL)
	if err != nil {
		return false, err
	}
	var ref plumbing.ReferenceName
	if refPlain != "" {
		ref = git.ParseRef(refPlain)
	}
	includeRepo := ref == "" || (!ref.IsTag() && !plumbing.IsHash(refPlain))
	_, inPrevious := previousRepos[repoURL]
	return includeRepo || !inPrevious, nil
}

func includeDifferentialV1beta1Repository(repo v1beta1.Repository, previousRepos []v1beta1.Repository) bool {
	if repo.Ref == nil || *repo.Ref == (v1beta1.GitRef{}) || repo.Ref.Branch != "" {
		return true
	}
	return !slices.ContainsFunc(previousRepos, func(previousRepo v1beta1.Repository) bool {
		return v1beta1RepositoriesEqual(repo, previousRepo)
	})
}

func v1beta1RepositoriesEqual(a, b v1beta1.Repository) bool {
	if a.URL != b.URL {
		return false
	}
	if a.Ref == nil || b.Ref == nil {
		return a.Ref == nil && b.Ref == nil
	}
	return *a.Ref == *b.Ref
}
