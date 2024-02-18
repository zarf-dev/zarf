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

// readZarfYAML reads a Zarf YAML file.
func (p *Packager) readZarfYAML(path string) error {
	var warnings []string

	if err := utils.ReadYaml(path, &p.cfg.Pkg); err != nil {
		return err
	}

	if p.layout.IsLegacyLayout() {
		warning := "Detected deprecated package layout, migrating to new layout - support for this package will be dropped in v1.0.0"
		p.warnings = append(p.warnings, warning)
	}

	if len(p.cfg.Pkg.Build.Migrations) > 0 {
		for idx, component := range p.cfg.Pkg.Components {
			// Handle component configuration deprecations
			p.cfg.Pkg.Components[idx], warnings = deprecated.MigrateComponent(p.cfg.Pkg.Build, component)
			p.warnings = append(p.warnings, warnings...)
		}
	}

	p.arch = config.GetArch(p.cfg.Pkg.Metadata.Architecture, p.cfg.Pkg.Build.Architecture)

	return nil
}

// filterComponentsByOSAndArch removes components not matching the current OS and architecture.
func (p *Packager) filterComponentsByOSAndArch() (err error) {
	p.cfg.Pkg.Components, err = filters.ByArchAndOS(p.arch, runtime.GOOS).Apply(p.cfg.Pkg)
	return err
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
	p.cfg.Pkg.Metadata.Architecture = p.arch
	p.cfg.Pkg.Build.Architecture = p.arch

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
	for _, m := range deprecated.Migrations() {
		p.cfg.Pkg.Build.Migrations = append(p.cfg.Pkg.Build.Migrations, m.ID())
	}

	// Record the flavor of Zarf used to build this package (if any).
	p.cfg.Pkg.Build.Flavor = p.cfg.CreateOpts.Flavor

	p.cfg.Pkg.Build.RegistryOverrides = p.cfg.CreateOpts.RegistryOverrides

	// Record the latest version of Zarf without breaking changes to the package structure.
	p.cfg.Pkg.Build.LastNonBreakingVersion = deprecated.LastNonBreakingVersion

	return utils.WriteYaml(p.layout.ZarfYAML, p.cfg.Pkg, 0400)
}
