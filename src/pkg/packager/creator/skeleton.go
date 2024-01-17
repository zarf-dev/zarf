// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/extensions/bigbang"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// veryify that SkeletonCreator implements Creator
	_ Creator = (*SkeletonCreator)(nil)
)

// SkeletonCreator provides methods for creating skeleton Zarf packages.
type SkeletonCreator struct {
	pkg        types.ZarfPackage
	createOpts types.ZarfCreateOptions
	layout     *layout.PackagePaths
	arch       string
	warnings   []string
}

// LoadPackageDefinition loads and configure a zarf.yaml file during package create.
func (sc *SkeletonCreator) LoadPackageDefinition() (pkg *types.ZarfPackage, err error) {
	if err := utils.ReadYaml(layout.ZarfYAML, &sc.pkg); err != nil {
		return nil, fmt.Errorf("unable to read the zarf.yaml file: %s", err.Error())
	}
	sc.arch = config.GetArch(sc.pkg.Metadata.Architecture, sc.pkg.Build.Architecture)

	if utils.IsInitConfig(sc.pkg) {
		sc.pkg.Metadata.Version = config.CLIVersion
	}

	if err := sc.setPackageBuildMetadata(); err != nil {
		message.Warn(err.Error())
	}

	// Compose components into a single zarf.yaml file
	sc.warnings, err = sc.ComposeComponents()
	if err != nil {
		return nil, err
	}

	return &sc.pkg, nil
}

// ComposeComponents builds the composed components list for the current config.
func (sc *SkeletonCreator) ComposeComponents() (warnings []string, err error) {
	return composeComponents(&sc.pkg, sc.createOpts)
}

// FillActiveTemplate handles setting the active variables and reloading the base template.
func (sc *SkeletonCreator) FillActiveTemplate() (warnings []string, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (sc *SkeletonCreator) ProcessExtensions() error {
	components := []types.ZarfComponent{}

	// Create component paths and process extensions for each component.
	for _, c := range sc.pkg.Components {
		componentPaths, err := sc.layout.Components.Create(c)
		if err != nil {
			return err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if c, err = bigbang.Skeletonize(componentPaths, c); err != nil {
				return fmt.Errorf("unable to process bigbang extension: %w", err)
			}
		}

		components = append(components, c)
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	sc.pkg.Components = components

	return nil
}

// LoadDifferentialData extracts the Zarf config of a designated 'reference' package,
// and creates a list of all images and repos that are in the reference package.
//
// This is not implemented.
func (sc *SkeletonCreator) LoadDifferentialData() error {
	return fmt.Errorf("not implemented")
}

// RemoveCopiesFromDifferentialPackage removes any images and repos already present in the reference package.
//
// This is not implemented.
func (sc *SkeletonCreator) RemoveCopiesFromDifferentialPackage() error {
	return fmt.Errorf("not implemented")
}

// setTempDirectory sets the temp directory for the SkeletonCreator.
func (sc *SkeletonCreator) setTempDirectory(path string) error {
	dir, err := utils.MakeTempDir(path)
	if err != nil {
		return fmt.Errorf("unable to create package temp paths: %w", err)
	}

	sc.layout = layout.New(dir)
	return nil
}

// setPackageBuildMetadata sets various package build metadata.
func (sc *SkeletonCreator) setPackageBuildMetadata() error {
	now := time.Now()
	// Just use $USER env variable to avoid CGO issue.
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
	// Record the name of the user creating the package.
	if runtime.GOOS == "windows" {
		sc.pkg.Build.User = os.Getenv("USERNAME")
	} else {
		sc.pkg.Build.User = os.Getenv("USER")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	sc.pkg.Metadata.Architecture = "skeleton"
	sc.pkg.Build.Architecture = "skeleton"

	// Record the time of package creation.
	sc.pkg.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with.
	sc.pkg.Build.Version = config.CLIVersion

	// Record the hostname of the package creation terminal.
	sc.pkg.Build.Terminal = hostname

	// Record the migrations that will be run on the package.
	sc.pkg.Build.Migrations = []string{
		deprecated.ScriptsToActionsMigrated,
		deprecated.PluralizeSetVariable,
	}

	// Record the flavor of Zarf used to build this package (if any).
	sc.pkg.Build.Flavor = sc.createOpts.Flavor

	sc.pkg.Build.RegistryOverrides = sc.createOpts.RegistryOverrides

	// Record the latest version of Zarf without breaking changes to the package structure.
	sc.pkg.Build.LastNonBreakingVersion = deprecated.LastNonBreakingVersion

	return nil
}
