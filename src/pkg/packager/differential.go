// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/mholt/archiver/v3"
)

// loadDifferentialData extracts the Zarf config of a designated 'reference' package used for building a differential package.
//
// It creates a list of all images and repositories that are in the reference package.
func (p *Packager) loadDifferentialData() error {
	// Save the fact that this is a differential build into the build data of the package
	p.cfg.Pkg.Build.Differential = true

	tmpDir, _ := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	defer os.RemoveAll(tmpDir)

	// Load the package spec of the package we're using as a 'reference' for the differential build
	if helpers.IsOCIURL(p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath) {
		remote, err := oci.NewOrasRemote(p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath)
		if err != nil {
			return err
		}
		pkg, err := remote.FetchZarfYAML()
		if err != nil {
			return err
		}
		err = utils.WriteYaml(filepath.Join(tmpDir, layout.ZarfYAML), pkg, 0600)
		if err != nil {
			return err
		}
	} else {
		if err := archiver.Extract(p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath, layout.ZarfYAML, tmpDir); err != nil {
			return fmt.Errorf("unable to extract the differential zarf package spec: %s", err.Error())
		}
	}

	var differentialZarfConfig types.ZarfPackage
	if err := utils.ReadYaml(filepath.Join(tmpDir, layout.ZarfYAML), &differentialZarfConfig); err != nil {
		return fmt.Errorf("unable to load the differential zarf package spec: %s", err.Error())
	}

	// Generate a map of all the images and repos that are included in the provided package
	allIncludedImagesMap := map[string]bool{}
	allIncludedReposMap := map[string]bool{}
	for _, component := range differentialZarfConfig.Components {
		for _, image := range component.Images {
			allIncludedImagesMap[image] = true
		}
		for _, repo := range component.Repos {
			allIncludedReposMap[repo] = true
		}
	}

	p.cfg.CreateOpts.DifferentialData.DifferentialImages = allIncludedImagesMap
	p.cfg.CreateOpts.DifferentialData.DifferentialRepos = allIncludedReposMap
	p.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion = differentialZarfConfig.Metadata.Version

	return nil
}

// removeCopiesFromDifferentialPackage removes any images and repositories that are already in the reference package from the new package.
//
// For each component in the new package, it checks if any images or repositories are duplicates from the reference package.
//
// Duplicate images and repositories are excluded, and the component lists are updated accordingly.
func (p *Packager) removeCopiesFromDifferentialPackage() error {
	// If a differential build was not requested, continue on as normal
	if p.cfg.CreateOpts.DifferentialData.DifferentialPackagePath == "" {
		return nil
	}

	// Loop through all of the components to determine if any of them are using already included images or repos
	componentMap := make(map[int]types.ZarfComponent)
	for idx, component := range p.cfg.Pkg.Components {
		newImageList := []string{}
		newRepoList := []string{}
		// Generate a list of all unique images for this component
		for _, img := range component.Images {
			// If a image doesn't have a ref (or is a commonly reused ref), we will include this image in the differential package
			imgRef, err := transform.ParseImageRef(img)
			if err != nil {
				return fmt.Errorf("unable to parse image ref %s: %s", img, err.Error())
			}

			// Only include new images or images that have a commonly overwritten tag
			imgTag := imgRef.TagOrDigest
			useImgAnyways := imgTag == ":latest" || imgTag == ":stable" || imgTag == ":nightly"
			if useImgAnyways || !p.cfg.CreateOpts.DifferentialData.DifferentialImages[img] {
				newImageList = append(newImageList, img)
			} else {
				message.Debugf("Image %s is already included in the differential package", img)
			}
		}

		// Generate a list of all unique repos for this component
		for _, repoURL := range component.Repos {
			// Split the remote url and the zarf reference
			_, refPlain, err := transform.GitURLSplitRef(repoURL)
			if err != nil {
				return err
			}

			var ref plumbing.ReferenceName
			// Parse the ref from the git URL.
			if refPlain != "" {
				ref = git.ParseRef(refPlain)
			}

			// Only include new repos or repos that were not referenced by a specific commit sha or tag
			useRepoAnyways := ref == "" || (!ref.IsTag() && !plumbing.IsHash(refPlain))
			if useRepoAnyways || !p.cfg.CreateOpts.DifferentialData.DifferentialRepos[repoURL] {
				newRepoList = append(newRepoList, repoURL)
			} else {
				message.Debugf("Repo %s is already included in the differential package", repoURL)
			}
		}

		// Update the component with the unique lists of repos and images
		component.Images = newImageList
		component.Repos = newRepoList
		componentMap[idx] = component
	}

	// Update the package with the new component list
	for idx, component := range componentMap {
		p.cfg.Pkg.Components[idx] = component
	}

	return nil
}
