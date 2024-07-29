// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import "github.com/zarf-dev/zarf/src/pkg/variables"

// ZarfPackageKind is an enum of the different kinds of Zarf packages.
type ZarfPackageKind string

const (
	// ZarfInitConfig is the kind of Zarf package used during `zarf init`.
	ZarfInitConfig ZarfPackageKind = "ZarfInitConfig"
	// ZarfPackageConfig is the default kind of Zarf package, primarily used during `zarf package`.
	ZarfPackageConfig ZarfPackageKind = "ZarfPackageConfig"
)

// ZarfPackage the top-level structure of a Zarf config file.
type ZarfPackage struct {
	// The kind of Zarf package
	Kind ZarfPackageKind `json:"kind" jsonschema:"required,enum=ZarfInitConfig,enum=ZarfPackageConfig,default=ZarfPackageConfig"`
	// Package metadata
	Metadata ZarfMetadata `json:"metadata"`
	// Zarf-generated package build data
	Build ZarfBuildData `json:"build"`
	// List of components to deploy in this package
	Components []ZarfComponent `json:"components" jsonschema:"required,minItems=1"`
	// Constant template values applied on deploy for K8s resources
	Constants []variables.Constant `json:"constants"`
	// Variable template values applied on deploy for K8s resources
	Variables []variables.InteractiveVariable `json:"variables"`
}

// IsInitConfig returns whether a Zarf package is an init config.
func (pkg ZarfPackage) IsInitConfig() bool {
	return pkg.Kind == ZarfInitConfig
}

// IsSBOMAble checks if a package has contents that an SBOM can be created on (i.e. images, files, or data injections).
func (pkg ZarfPackage) IsSBOMAble() bool {
	for _, c := range pkg.Components {
		if len(c.Images) > 0 || len(c.Files) > 0 || len(c.DataInjections) > 0 {
			return true
		}
	}
	return false
}

// ZarfMetadata lists information about the current ZarfPackage.
type ZarfMetadata struct {
	// Name to identify this Zarf package
	Name string `json:"name" jsonschema:"required,pattern=^[a-z0-9][a-z0-9\\-]*$"`
	// Additional information about this package
	Description string `json:"description"`
	// Generic string set by a package author to track the package version (Note: ZarfInitConfigs will always be versioned to the CLIVersion they were created with)
	Version string `json:"version"`
	// Link to package information when online
	URL string `json:"url"`
	// An image URL to embed in this package (Reserved for future use in Zarf UI)
	Image string `json:"image"`
	// Disable compression of this package
	Uncompressed bool `json:"uncompressed"`
	// The target cluster architecture for this package
	Architecture string `json:"architecture" jsonschema:"example=arm64,example=amd64"`
	// Yaml OnLy Online (YOLO): True enables deploying a Zarf package without first running zarf init against the cluster. This is ideal for connected environments where you want to use existing VCS and container registries.
	YOLO bool `json:"yolo"`
	// Comma-separated list of package authors (including contact info)
	Authors string `json:"authors" jsonschema:"example=Doug &#60;hello@defenseunicorns.com&#62;&#44; Pepr &#60;hello@defenseunicorns.com&#62;"`
	// Link to package documentation when online
	Documentation string `json:"documentation"`
	// Link to package source code when online
	Source string `json:"source"`
	// Name of the distributing entity, organization or individual
	Vendor string `json:"vendor"`
	// Checksum of a checksums.txt file that contains checksums all the layers within the package
	AggregateChecksum string `json:"aggregateChecksum"`
}

// ZarfBuildData is written during the packager.Create() operation to track details of the created package.
type ZarfBuildData struct {
	// The machine name that created this package
	Terminal string `json:"terminal" jsonschema:"required"`
	// The username who created this package
	User string `json:"user" jsonschema:"required"`
	// The architecture this package was created on
	Architecture string `json:"architecture" jsonschema:"required"`
	// The timestamp when this package was created
	Timestamp string `json:"timestamp" jsonschema:"required"`
	// The version of Zarf used to build this package
	Version string `json:"version" jsonschema:"required"`
	// Any migrations that have been run on this package
	Migrations []string `json:"migrations"`
	// Any registry domains that were overridden on package create when pulling images
	RegistryOverrides map[string]string `json:"registryOverrides"`
	// Whether this package was created with differential components
	Differential bool `json:"differential"`
	// Version of a previously built package used as the basis for creating this differential package
	DifferentialPackageVersion string `json:"differentialPackageVersion"`
	// List of components that were not included in this package due to differential packaging
	DifferentialMissing []string `json:"differentialMissing"`
	// The minimum version of Zarf that does not have breaking package structure changes
	LastNonBreakingVersion string `json:"lastNonBreakingVersion"`
	// The flavor of Zarf used to build this package
	Flavor string `json:"flavor"`
}
