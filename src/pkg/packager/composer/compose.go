// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

func ComposeComponents(zarfPackage *types.ZarfPackage, createOpts types.ZarfCreateOptions,
	warnings []string) ([]string, error) {
	components := []types.ZarfComponent{}

	pkgVars := zarfPackage.Variables
	pkgConsts := zarfPackage.Constants

	for i, component := range zarfPackage.Components {
		//TODO allow this to be a CLI option
		arch := config.GetArch(zarfPackage.Metadata.Architecture)

		// filter by architecture
		if !CompatibleComponent(component, arch, createOpts.Flavor) {
			continue
		}

		// if a match was found, strip flavor and architecture to reduce bloat in the package definition
		component.Only.Cluster.Architecture = ""
		component.Only.Flavor = ""

		// build the import chain
		chain, err := NewImportChain(component, i, arch, createOpts.Flavor)
		if err != nil {
			return warnings, err
		}
		message.Debugf("%s", chain)

		// migrate any deprecated component configurations now
		warnings := chain.Migrate(zarfPackage.Build)
		warnings = append(warnings, warnings...)

		// get the composed component
		composed, err := chain.Compose()
		if err != nil {
			return warnings, err
		}
		components = append(components, composed)

		// merge variables and constants
		pkgVars = chain.MergeVariables(pkgVars)
		pkgConsts = chain.MergeConstants(pkgConsts)
	}

	// set the filtered + composed components
	zarfPackage.Components = components

	zarfPackage.Variables = pkgVars
	zarfPackage.Constants = pkgConsts

	return warnings, nil
}
