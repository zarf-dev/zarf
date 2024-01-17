// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/extensions/bigbang"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/mholt/archiver"
)

var (
	// veryify that PackageCreator implements Creator
	_ Creator = (*PackageCreator)(nil)
)

// PackageCreator provides methods for creating normal (not skeleton) Zarf packages.
type PackageCreator struct {
	pkg        types.ZarfPackage
	createOpts types.ZarfCreateOptions
	layout     *layout.PackagePaths
	arch       string
	warnings   []string
}

// LoadPackageDefinition loads and configure a zarf.yaml file during package create.
func (pc *PackageCreator) LoadPackageDefinition() (pkg *types.ZarfPackage, err error) {
	if err := utils.ReadYaml(layout.ZarfYAML, &pc.pkg); err != nil {
		return nil, fmt.Errorf("unable to read the zarf.yaml file: %s", err.Error())
	}
	pc.arch = config.GetArch()

	if utils.IsInitConfig(pc.pkg) {
		pc.pkg.Metadata.Version = config.CLIVersion
	}

	if err := pc.setPackageBuildMetadata(); err != nil {
		message.Warn(err.Error())
	}

	// Compose components into a single zarf.yaml file
	if pc.warnings, err = pc.ComposeComponents(); err != nil {
		return nil, err
	}

	// After components are composed, template the active package.
	if pc.warnings, err = pc.FillActiveTemplate(); err != nil {
		return nil, fmt.Errorf("unable to fill values in template: %s", err.Error())
	}

	// After templates are filled process any create extensions
	if err := pc.ProcessExtensions(); err != nil {
		return nil, err
	}

	// After we have a full zarf.yaml remove unnecessary repos and images if we are building a differential package
	if pc.createOpts.DifferentialData.DifferentialPackagePath != "" {
		// Load the images and repos from the 'reference' package
		if err := pc.LoadDifferentialData(); err != nil {
			return nil, err
		}
		// Verify the package version of the package we're using as a 'reference' for the differential build is different than the package we're building
		// If the package versions are the same return an error
		if pc.createOpts.DifferentialData.DifferentialPackageVersion == pc.pkg.Metadata.Version {
			return nil, errors.New(lang.PkgCreateErrDifferentialSameVersion)
		}
		if pc.createOpts.DifferentialData.DifferentialPackageVersion == "" || pc.pkg.Metadata.Version == "" {
			return nil, fmt.Errorf("unable to build differential package when either the differential package version or the referenced package version is not set")
		}

		// Handle any potential differential images/repos before going forward
		if err := pc.RemoveCopiesFromDifferentialPackage(); err != nil {
			return nil, err
		}
	}

	return &pc.pkg, nil
}

// ComposeComponents builds the composed components list for the current config.
func (pc *PackageCreator) ComposeComponents() (warnings []string, err error) {
	return composeComponents(&pc.pkg, pc.createOpts)
}

// FillActiveTemplate handles setting the active variables and reloading the base template.
func (pc *PackageCreator) FillActiveTemplate() (warnings []string, err error) {
	return fillActiveTemplate(&pc.pkg, pc.createOpts)
}

func (pc *PackageCreator) ProcessExtensions() error {
	components := []types.ZarfComponent{}

	// Create component paths and process extensions for each component.
	for _, c := range pc.pkg.Components {
		componentPaths, err := pc.layout.Components.Create(c)
		if err != nil {
			return err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if c, err = bigbang.Run(pc.pkg.Metadata.YOLO, componentPaths, c); err != nil {
				return fmt.Errorf("unable to process bigbang extension: %w", err)
			}
		}

		components = append(components, c)
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	pc.pkg.Components = components

	return nil
}

// LoadDifferentialData extracts the Zarf config of a designated 'reference' package,
// and creates a list of all images and repos that are in the reference package.
func (pc *PackageCreator) LoadDifferentialData() error {
	// Save the fact that this is a differential build into the build data of the package
	pc.pkg.Build.Differential = true

	tmpDir, _ := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	defer os.RemoveAll(tmpDir)

	// Load the package spec of the package we're using as a 'reference' for the differential build
	if helpers.IsOCIURL(pc.createOpts.DifferentialData.DifferentialPackagePath) {
		remote, err := oci.NewOrasRemote(pc.createOpts.DifferentialData.DifferentialPackagePath)
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
		if err := archiver.Extract(pc.createOpts.DifferentialData.DifferentialPackagePath, layout.ZarfYAML, tmpDir); err != nil {
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

	pc.createOpts.DifferentialData.DifferentialImages = allIncludedImagesMap
	pc.createOpts.DifferentialData.DifferentialRepos = allIncludedReposMap
	pc.createOpts.DifferentialData.DifferentialPackageVersion = differentialZarfConfig.Metadata.Version

	return nil
}

// RemoveCopiesFromDifferentialPackage removes any images and repos already present in the reference package.
func (pc *PackageCreator) RemoveCopiesFromDifferentialPackage() error {
	// If a differential build was not requested, continue on as normal
	if pc.createOpts.DifferentialData.DifferentialPackagePath == "" {
		return nil
	}

	// Loop through all of the components to determine if any of them are using already included images or repos
	componentMap := make(map[int]types.ZarfComponent)
	for idx, component := range pc.pkg.Components {
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
			if useImgAnyways || !pc.createOpts.DifferentialData.DifferentialImages[img] {
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
			if useRepoAnyways || !pc.createOpts.DifferentialData.DifferentialRepos[repoURL] {
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
		pc.pkg.Components[idx] = component
	}

	return nil
}

// setTempDirectory sets the temp directory for the PackageCreator.
func (pc *PackageCreator) setTempDirectory(path string) error {
	dir, err := utils.MakeTempDir(path)
	if err != nil {
		return fmt.Errorf("unable to create package temp paths: %w", err)
	}

	pc.layout = layout.New(dir)
	return nil
}

// setPackageBuildMetadata sets various package build metadata.
func (pc *PackageCreator) setPackageBuildMetadata() error {
	now := time.Now()
	// Just use $USER env variable to avoid CGO issue.
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
	// Record the name of the user creating the package.
	if runtime.GOOS == "windows" {
		pc.pkg.Build.User = os.Getenv("USERNAME")
	} else {
		pc.pkg.Build.User = os.Getenv("USER")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	pc.pkg.Metadata.Architecture = pc.arch
	pc.pkg.Build.Architecture = pc.arch

	// Record the time of package creation.
	pc.pkg.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with.
	pc.pkg.Build.Version = config.CLIVersion

	// Record the hostname of the package creation terminal.
	pc.pkg.Build.Terminal = hostname

	// Record the migrations that will be run on the package.
	pc.pkg.Build.Migrations = []string{
		deprecated.ScriptsToActionsMigrated,
		deprecated.PluralizeSetVariable,
	}

	// Record the flavor of Zarf used to build this package (if any).
	pc.pkg.Build.Flavor = pc.createOpts.Flavor

	pc.pkg.Build.RegistryOverrides = pc.createOpts.RegistryOverrides

	// Record the latest version of Zarf without breaking changes to the package structure.
	pc.pkg.Build.LastNonBreakingVersion = deprecated.LastNonBreakingVersion

	return nil
}
