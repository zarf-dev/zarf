// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-git/go-git/v5/plumbing"
)

// loadDifferentialData sets any images and repos from the existing reference package in the DifferentialData and returns it.
func loadDifferentialData(diffPkgPath string) (diffData *types.DifferentialData, err error) {
	tmpdir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}

	diffLayout := layout.New(tmpdir)
	defer os.RemoveAll(diffLayout.Base)

	src, err := sources.New(&types.ZarfPackageOptions{
		PackageSource: diffPkgPath,
	})
	if err != nil {
		return nil, err
	}

	diffPkg, _, err := src.LoadPackageMetadata(diffLayout, false, false)
	if err != nil {
		return nil, err
	}

	allIncludedImagesMap := map[string]bool{}
	allIncludedReposMap := map[string]bool{}

	for _, component := range diffPkg.Components {
		for _, image := range component.Images {
			allIncludedImagesMap[image] = true
		}
		for _, repo := range component.Repos {
			allIncludedReposMap[repo] = true
		}
	}

	return &types.DifferentialData{
		DifferentialImages:         allIncludedImagesMap,
		DifferentialRepos:          allIncludedReposMap,
		DifferentialPackageVersion: diffPkg.Metadata.Version,
	}, nil
}

// removeCopiesFromComponents removes any images and repos already present in the reference package components.
func removeCopiesFromComponents(components []types.ZarfComponent, loadedDiffData *types.DifferentialData) (diffComponents []types.ZarfComponent, err error) {
	for _, component := range components {
		newImageList := []string{}
		newRepoList := []string{}

		for _, img := range component.Images {
			imgRef, err := transform.ParseImageRef(img)
			if err != nil {
				return nil, fmt.Errorf("unable to parse image ref %s: %s", img, err.Error())
			}

			imgTag := imgRef.TagOrDigest
			includeImage := imgTag == ":latest" || imgTag == ":stable" || imgTag == ":nightly"
			if includeImage || !loadedDiffData.DifferentialImages[img] {
				newImageList = append(newImageList, img)
			}
		}

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
			if includeRepo || !loadedDiffData.DifferentialRepos[repoURL] {
				newRepoList = append(newRepoList, repoURL)
			}
		}

		component.Images = newImageList
		component.Repos = newRepoList
		diffComponents = append(diffComponents, component)
	}

	return diffComponents, nil
}
