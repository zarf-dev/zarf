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
func setPackageMetadata(pkg types.ZarfPackage, createOpts types.ZarfCreateOptions) (configuredPkg types.ZarfPackage, err error) {
	configuredPkg = pkg

	now := time.Now()
	// Just use $USER env variable to avoid CGO issue.
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
	// Record the name of the user creating the package.
	if runtime.GOOS == "windows" {
		configuredPkg.Build.User = os.Getenv("USERNAME")
	} else {
		configuredPkg.Build.User = os.Getenv("USER")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return configuredPkg, err
	}

	if utils.IsInitConfig(pkg) {
		configuredPkg.Metadata.Version = config.CLIVersion
	}

	// Set package architecture
	if createOpts.IsSkeleton {
		configuredPkg.Metadata.Architecture = "skeleton"
	}
	if configuredPkg.Metadata.Architecture == "" {
		configuredPkg.Metadata.Architecture = config.GetArch()
	}
	configuredPkg.Build.Architecture = configuredPkg.Metadata.Architecture

	// Record the time of package creation.
	configuredPkg.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with.
	configuredPkg.Build.Version = config.CLIVersion

	// Record the hostname of the package creation terminal.
	configuredPkg.Build.Terminal = hostname

	// If the --differential flag was used, record that this is a differential package.
	if createOpts.DifferentialData.DifferentialPackagePath != "" {
		configuredPkg.Build.Differential = true
	}

	// Record the flavor of Zarf used to build this package (if any).
	configuredPkg.Build.Flavor = createOpts.Flavor

	configuredPkg.Build.RegistryOverrides = createOpts.RegistryOverrides

	// Record the latest version of Zarf without breaking changes to the package structure.
	configuredPkg.Build.LastNonBreakingVersion = deprecated.LastNonBreakingVersion

	return configuredPkg, nil
}
