// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"runtime"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

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
	return utils.WriteYaml(p.layout.ZarfYAML, p.cfg.Pkg, 0400)
}
