// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/composer"
	"github.com/defenseunicorns/zarf/src/types"
)

// ComposeComponents composes components and their dependencies into a single Zarf package using an import chain.
func ComposeComponents(pkg types.ZarfPackage, flavor string) (types.ZarfPackage, []string, error) {
	components := []types.ZarfComponent{}
	warnings := []string{}

	pkgVars := pkg.Variables
	pkgConsts := pkg.Constants

	arch := pkg.Metadata.Architecture

	for i, component := range pkg.Components {
		// filter by architecture and flavor
		if !composer.CompatibleComponent(component, arch, flavor) {
			continue
		}

		// if a match was found, strip flavor and architecture to reduce bloat in the package definition
		component.Only.Cluster.Architecture = ""
		component.Only.Flavor = ""

		// build the import chain
		chain, err := composer.NewImportChain(component, i, pkg.Metadata.Name, arch, flavor)
		if err != nil {
			return types.ZarfPackage{}, nil, err
		}
		message.Debugf("%s", chain)

		// migrate any deprecated component configurations now
		warning := chain.Migrate(pkg.Build)
		warnings = append(warnings, warning...)

		// get the composed component
		composed, err := chain.Compose()
		if err != nil {
			return types.ZarfPackage{}, nil, err
		}
		components = append(components, *composed)

		// merge variables and constants
		pkgVars = chain.MergeVariables(pkgVars)
		pkgConsts = chain.MergeConstants(pkgConsts)
	}

	// set the filtered + composed components
	pkg.Components = components

	pkg.Variables = pkgVars
	pkg.Constants = pkgConsts

	return pkg, warnings, nil
}
