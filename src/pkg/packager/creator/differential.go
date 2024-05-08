// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
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
