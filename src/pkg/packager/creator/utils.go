// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"os"
	"runtime"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// setPackageMetadata sets various package metadata.
func setPackageMetadata(pkg *types.ZarfPackage, createOpts types.ZarfCreateOptions) (*types.ZarfPackage, error) {
	now := time.Now()
	// Just use $USER env variable to avoid CGO issue.
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
	// Record the name of the user creating the package.
	if runtime.GOOS == "windows" {
		pkg.Build.User = os.Getenv("USERNAME")
	} else {
		pkg.Build.User = os.Getenv("USER")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	if utils.IsInitConfig(*pkg) {
		pkg.Metadata.Version = config.CLIVersion
	}

	// Set package architecture
	if createOpts.IsSkeleton {
		pkg.Metadata.Architecture = "skeleton"
	}
	if pkg.Metadata.Architecture == "" {
		pkg.Metadata.Architecture = config.GetArch()
	}
	pkg.Build.Architecture = pkg.Metadata.Architecture

	// Record the time of package creation.
	pkg.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with.
	pkg.Build.Version = config.CLIVersion

	// Record the hostname of the package creation terminal.
	pkg.Build.Terminal = hostname

	// If the --differential flag was used, record that this is a differential package.
	if createOpts.DifferentialData.DifferentialPackagePath != "" {
		pkg.Build.Differential = true
	}

	// Record the migrations that will be run on the package.
	pkg.Build.Migrations = []string{
		deprecated.ScriptsToActionsMigrated,
		deprecated.PluralizeSetVariable,
	}

	// Record the flavor of Zarf used to build this package (if any).
	pkg.Build.Flavor = createOpts.Flavor

	pkg.Build.RegistryOverrides = createOpts.RegistryOverrides

	// Record the latest version of Zarf without breaking changes to the package structure.
	pkg.Build.LastNonBreakingVersion = deprecated.LastNonBreakingVersion

	return pkg, nil
}
