// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters

import (
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

// ByDifferentialData filters any images and repos already present in the reference package components.
func ByDifferentialData(images map[string]bool, repos map[string]bool) ComponentFilterStrategy {
	return &differentialDataFilter{
		images: images,
		repos:  repos,
	}
}

type differentialDataFilter struct {
	images map[string]bool
	repos  map[string]bool
}

func (f *differentialDataFilter) Apply(pkg v1alpha1.ZarfPackage) ([]v1alpha1.ZarfComponent, error) {
	diffComponents := []v1alpha1.ZarfComponent{}
	for _, component := range pkg.Components {
		filteredImages := []string{}
		for _, img := range component.Images {
			imgRef, err := transform.ParseImageRef(img)
			if err != nil {
				return nil, fmt.Errorf("unable to parse image ref %s: %w", img, err)
			}
			imgTag := imgRef.TagOrDigest
			includeImage := imgTag == ":latest" || imgTag == ":stable" || imgTag == ":nightly"
			if includeImage || !f.images[img] {
				filteredImages = append(filteredImages, img)
			}
		}
		component.Images = filteredImages

		filteredRepos := []string{}
		for _, repoURL := range component.Repos {
			_, refPlain, err := transform.GitURLSplitRef(repoURL)
			if err != nil {
				return nil, err
			}
			var ref plumbing.ReferenceName
			if refPlain != "" {
				ref = git.ParseRef(refPlain)
			}
			includeRepo := ref == "" || (!ref.IsTag() && !plumbing.IsHash(refPlain))
			if includeRepo || !f.repos[repoURL] {
				filteredRepos = append(filteredRepos, repoURL)
			}
		}
		component.Repos = filteredRepos

		diffComponents = append(diffComponents, component)
	}
	return diffComponents, nil
}
