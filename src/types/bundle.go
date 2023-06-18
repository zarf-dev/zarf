// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// ZarfBundle is the top-level structure of a Zarf bundle file.
type ZarfBundle struct {
	Metadata ZarfMetadata        `json:"metadata" jsonschema:"description=Bundle metadata"`
	Build    ZarfBuildData       `json:"build,omitempty" jsonschema:"description=Zarf-generated bundle build data"`
	Packages []ZarfPackageImport `json:"packages" jsonschema:"description=List of packages to import"`
}

// ZarfPackageImport is a package import statement in a Zarf bundle file.
type ZarfPackageImport struct {
	Repository string   `json:"repository" jsonschema:"description=The repository to import the package from"`
	Ref        string   `json:"ref"`
	Components []string `json:"components,omitempty" jsonschema:"description=List of components to include from the package"`
}
