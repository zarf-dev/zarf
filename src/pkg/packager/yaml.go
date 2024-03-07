// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"os"
	"runtime"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

func (p *Packager) archAndOSFilter() filters.ComponentFilterStrategy {
	return filters.ByArchAndOS(p.pkgArch(), runtime.GOOS)
}

// writeYaml adds build information and writes the config to the temp directory.
func (p *Packager) writeYaml() error {
	now := time.Now()
	// Just use $USER env variable to avoid CGO issue.
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
	// Record the name of the user creating the package.
	if runtime.GOOS == "windows" {
		p.cfg.Pkg.Build.User = os.Getenv("USERNAME")
	} else {
		p.cfg.Pkg.Build.User = os.Getenv("USER")
	}
	hostname, hostErr := os.Hostname()

	// Normalize these for the package confirmation.
	arch := p.pkgArch()
	p.cfg.Pkg.Metadata.Architecture = arch
	p.cfg.Pkg.Build.Architecture = arch

	if p.cfg.CreateOpts.IsSkeleton {
		p.cfg.Pkg.Build.Architecture = "skeleton"
	}

	// Record the time of package creation.
	p.cfg.Pkg.Build.Timestamp = now.Format(time.RFC1123Z)

	// Record the Zarf Version the CLI was built with.
	p.cfg.Pkg.Build.Version = config.CLIVersion

	if hostErr == nil {
		// Record the hostname of the package creation terminal.
		p.cfg.Pkg.Build.Terminal = hostname
	}

	// Record the migrations that will be run on the package.
	p.cfg.Pkg.Build.Migrations = []string{
		deprecated.ScriptsToActionsMigrated,
		deprecated.PluralizeSetVariable,
	}

	// Record the flavor of Zarf used to build this package (if any).
	p.cfg.Pkg.Build.Flavor = p.cfg.CreateOpts.Flavor

	p.cfg.Pkg.Build.RegistryOverrides = p.cfg.CreateOpts.RegistryOverrides

	// Record the latest version of Zarf without breaking changes to the package structure.
	p.cfg.Pkg.Build.LastNonBreakingVersion = deprecated.LastNonBreakingVersion

	return utils.WriteYaml(p.layout.ZarfYAML, p.cfg.Pkg, 0400)
}
