// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// ZarfBundle is the top-level structure of a Zarf bundle file.
type ZarfBundle struct {
	Metadata  ZarfMetadata          `json:"metadata,omitempty" jsonschema:"description=Bundle metadata"`
	Build     ZarfBuildData         `json:"build,omitempty" jsonschema:"description=Zarf-generated bundle build data"`
	Packages  []ZarfPackageImport   `json:"packages" jsonschema:"description=List of packages to import"`
	Variables []ZarfPackageVariable `json:"variables,omitempty" jsonschema:"description=Variable template values applied on deploy for K8s resources"`
	Constants []ZarfPackageConstant `json:"constants,omitempty" jsonschema:"description=Constant template values applied on deploy for K8s resources"`
}

type ZarfPackageImport struct {
	Repository string   `json:"repository" jsonschema:"description=The repository to import the package from"`
	Ref        string   `json:"ref"`
	Components []string `json:"components,omitempty" jsonschema:"description=List of components to include from the package"`
}
