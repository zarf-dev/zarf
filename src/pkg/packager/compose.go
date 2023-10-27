// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/composer"
	"github.com/defenseunicorns/zarf/src/types"
)

// composeComponents builds the composed components list for the current config.
func (p *Packager) composeComponents() error {
	components := []types.ZarfComponent{}

	pkgVars := p.cfg.Pkg.Variables
	pkgConsts := p.cfg.Pkg.Constants

	for _, component := range p.cfg.Pkg.Components {
		arch := p.arch
		// filter by architecture
		if (component.Only.Cluster.Architecture != "" && component.Only.Cluster.Architecture != arch) ||
			(component.Only.Flavor != "" && component.Only.Flavor != p.cfg.CreateOpts.Flavor) {
			continue
		}

		// build the import chain
		chain, err := composer.NewImportChain(component, arch, p.cfg.CreateOpts.Flavor)
		if err != nil {
			return err
		}
		message.Debugf("%s", chain)

		// migrate any deprecated component configurations now
		warnings := chain.Migrate(p.cfg.Pkg.Build)
		p.warnings = append(p.warnings, warnings...)

		// get the composed component
		composed, err := chain.Compose()
		if err != nil {
			return err
		}
		components = append(components, composed)

		// merge variables and constants
		pkgVars = chain.MergeVariables(pkgVars)
		pkgConsts = chain.MergeConstants(pkgConsts)
	}

	// set the filtered + composed components
	p.cfg.Pkg.Components = components

	p.cfg.Pkg.Variables = pkgVars
	p.cfg.Pkg.Constants = pkgConsts

	return nil
}
