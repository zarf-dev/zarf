// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"os"
	"runtime"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// ReadZarfYAML reads a Zarf YAML file.
func ReadZarfYAML(path string) (pkg types.ZarfPackage, arch string, err error) {
	if err := utils.ReadYaml(path, &pkg); err != nil {
		return pkg, "", err
	}
	if pkg.Build.OCIImportedComponents == nil {
		pkg.Build.OCIImportedComponents = make(map[string]string)
	}
	return pkg, config.GetArch(pkg.Metadata.Architecture, pkg.Build.Architecture), nil
}

// filterComponents removes components not matching the current OS if filterByOS is set.
func (p *Packager) filterComponents() {
	// Filter each component to only compatible platforms.
	filteredComponents := []types.ZarfComponent{}
	for _, component := range p.cfg.Pkg.Components {
		// Ignore only filters that are empty
		var validArch, validOS bool

		// Test for valid architecture
		if component.Only.Cluster.Architecture == "" || component.Only.Cluster.Architecture == p.arch {
			validArch = true
		} else {
			message.Debugf("Skipping component %s, %s is not compatible with %s", component.Name, component.Only.Cluster.Architecture, p.arch)
		}

		// Test for a valid OS
		if component.Only.LocalOS == "" || component.Only.LocalOS == runtime.GOOS {
			validOS = true
		} else {
			message.Debugf("Skipping component %s, %s is not compatible with %s", component.Name, component.Only.LocalOS, runtime.GOOS)
		}

		// If both the OS and architecture are valid, add the component to the filtered list
		if validArch && validOS {
			filteredComponents = append(filteredComponents, component)
		}
	}
	// Update the active package with the filtered components.
	p.cfg.Pkg.Components = filteredComponents
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

	p.cfg.Pkg.Build.RegistryOverrides = p.cfg.CreateOpts.RegistryOverrides

	// Record the latest version of Zarf without breaking changes to the package structure.
	p.cfg.Pkg.Build.LastNonBreakingVersion = deprecated.LastNonBreakingVersion

	return utils.WriteYaml(p.layout.ZarfYAML, p.cfg.Pkg, 0400)
}
