// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/composer"
	"github.com/defenseunicorns/zarf/src/types"
)

func ComposeComponents(pkg types.ZarfPackage, createOpts types.ZarfCreateOptions) (composedPkg types.ZarfPackage, warnings []string, err error) {
	composedPkg = pkg

	components := []types.ZarfComponent{}
	pkgVars := []types.ZarfPackageVariable{}
	pkgConsts := []types.ZarfPackageConstant{}

	for i, component := range pkg.Components {
		arch := pkg.Metadata.Architecture
		// filter by architecture
		if !composer.CompatibleComponent(component, arch, createOpts.Flavor) {
			continue
		}

		// if a match was found, strip flavor and architecture to reduce bloat in the package definition
		component.Only.Cluster.Architecture = ""
		component.Only.Flavor = ""

		// build the import chain
		chain, err := composer.NewImportChain(component, i, pkg.Metadata.Name, arch, createOpts.Flavor)
		if err != nil {
			return pkg, nil, err
		}
		message.Debugf("%s", chain)

		// migrate any deprecated component configurations now
		migrationWarnings := chain.Migrate(pkg.Build)
		warnings = append(warnings, migrationWarnings...)

		// get the composed component
		composed, err := chain.Compose()
		if err != nil {
			return pkg, nil, err
		}
		components = append(components, *composed)

		// merge variables and constants
		pkgVars = chain.MergeVariables(pkgVars)
		pkgConsts = chain.MergeConstants(pkgConsts)
	}

	// set the filtered + composed components
	composedPkg.Components = components

	composedPkg.Variables = pkgVars
	composedPkg.Constants = pkgConsts

	return composedPkg, warnings, nil
}
