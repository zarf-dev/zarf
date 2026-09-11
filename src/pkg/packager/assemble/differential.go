// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package assemble

import (
	"fmt"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func applyDifferentialResources(definition, previous api.Package) (api.Package, error) {
	if !apiVersionsMatch(definition.APIVersion, previous.APIVersion) {
		return api.Package{}, fmt.Errorf("package apiVersion %s does not match differential package apiVersion %s", normalizeAPIVersion(definition.APIVersion), normalizeAPIVersion(previous.APIVersion))
	}

	definition.Components = slices.Clone(definition.Components)
	previousImages, previousRepos := differentialResources(previous.Components)
	for componentIdx := range definition.Components {
		component := &definition.Components[componentIdx]
		images := make([]api.Image, 0, len(component.Images))
		for _, image := range component.Images {
			includeImage, err := includeDifferentialImage(image.Name, previousImages)
			if err != nil {
				return api.Package{}, err
			}
			if includeImage {
				images = append(images, image)
			}
		}
		component.Images = images

		repositories := make([]api.Repository, 0, len(component.Repositories))
		for _, repository := range component.Repositories {
			includeRepository, err := includeDifferentialRepository(repository, previousRepos)
			if err != nil {
				return api.Package{}, err
			}
			if includeRepository {
				repositories = append(repositories, repository)
			}
		}
		component.Repositories = repositories
	}
	return definition, nil
}

func apiVersionsMatch(first, second string) bool {
	return normalizeAPIVersion(first) == normalizeAPIVersion(second)
}

func normalizeAPIVersion(apiVersion string) string {
	if apiVersion == "" {
		return v1alpha1.APIVersion
	}
	return apiVersion
}

func differentialResources(components []api.Component) (map[string]struct{}, []api.Repository) {
	images := map[string]struct{}{}
	var repositories []api.Repository
	for _, component := range components {
		for _, image := range component.Images {
			images[image.Name] = struct{}{}
		}
		repositories = append(repositories, component.Repositories...)
	}
	return images, repositories
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

func includeDifferentialRepository(repository api.Repository, previousRepositories []api.Repository) (bool, error) {
	if repository.Ref != nil {
		if *repository.Ref == (api.GitRef{}) || repository.Ref.Branch != "" {
			return true, nil
		}
		return !slices.ContainsFunc(previousRepositories, func(previousRepository api.Repository) bool {
			return repositoriesEqual(repository, previousRepository)
		}), nil
	}

	_, refPlain, err := transform.GitURLSplitRef(repository.URL)
	if err != nil {
		return false, err
	}
	var ref plumbing.ReferenceName
	if refPlain != "" {
		ref = git.ParseRef(refPlain)
	}
	includeRepo := ref == "" || (!ref.IsTag() && !plumbing.IsHash(refPlain))
	inPrevious := slices.ContainsFunc(previousRepositories, func(previousRepository api.Repository) bool {
		return repositoriesEqual(repository, previousRepository)
	})
	return includeRepo || !inPrevious, nil
}

func repositoriesEqual(a, b api.Repository) bool {
	if a.URL != b.URL {
		return false
	}
	if a.Ref == nil || b.Ref == nil {
		return a.Ref == nil && b.Ref == nil
	}
	return *a.Ref == *b.Ref
}
