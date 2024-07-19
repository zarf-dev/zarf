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
	Kind       ZarfPackageKind                 `json:"kind" jsonschema:"description=The kind of Zarf package,enum=ZarfInitConfig,enum=ZarfPackageConfig,default=ZarfPackageConfig"`
	Metadata   ZarfMetadata                    `json:"metadata,omitempty" jsonschema:"description=Package metadata"`
	Build      ZarfBuildData                   `json:"build,omitempty" jsonschema:"description=Zarf-generated package build data"`
	Components []ZarfComponent                 `json:"components" jsonschema:"description=List of components to deploy in this package,minItems=1"`
	Constants  []variables.Constant            `json:"constants,omitempty" jsonschema:"description=Constant template values applied on deploy for K8s resources"`
	Variables  []variables.InteractiveVariable `json:"variables,omitempty" jsonschema:"description=Variable template values applied on deploy for K8s resources"`
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
	Name              string `json:"name" jsonschema:"description=Name to identify this Zarf package,pattern=^[a-z0-9][a-z0-9\\-]*$"`
	Description       string `json:"description,omitempty" jsonschema:"description=Additional information about this package"`
	Version           string `json:"version,omitempty" jsonschema:"description=Generic string set by a package author to track the package version (Note: ZarfInitConfigs will always be versioned to the CLIVersion they were created with)"`
	URL               string `json:"url,omitempty" jsonschema:"description=Link to package information when online"`
	Uncompressed      bool   `json:"uncompressed,omitempty" jsonschema:"description=Disable compression of this package"`
	Architecture      string `json:"architecture,omitempty" jsonschema:"description=The target cluster architecture for this package,example=arm64,example=amd64"`
	YOLO              bool   `json:"yolo,omitempty" jsonschema:"description=Yaml OnLy Online (YOLO): True enables deploying a Zarf package without first running zarf init against the cluster. This is ideal for connected environments where you want to use existing VCS and container registries."`
	Authors           string `json:"authors,omitempty" jsonschema:"description=Comma-separated list of package authors (including contact info),example=Doug &#60;hello@defenseunicorns.com&#62;&#44; Pepr &#60;hello@defenseunicorns.com&#62;"`
	Documentation     string `json:"documentation,omitempty" jsonschema:"description=Link to package documentation when online"`
	Source            string `json:"source,omitempty" jsonschema:"description=Link to package source code when online"`
	Vendor            string `json:"vendor,omitempty" jsonschema_description:"Name of the distributing entity, organization or individual."`
	AggregateChecksum string `json:"aggregateChecksum,omitempty" jsonschema:"description=Checksum of a checksums.txt file that contains checksums all the layers within the package."`
}

// ZarfBuildData is written during the packager.Create() operation to track details of the created package.
type ZarfBuildData struct {
	Terminal                   string            `json:"terminal" jsonschema:"description=The machine name that created this package"`
	User                       string            `json:"user" jsonschema:"description=The username who created this package"`
	Architecture               string            `json:"architecture" jsonschema:"description=The architecture this package was created on"`
	Timestamp                  string            `json:"timestamp" jsonschema:"description=The timestamp when this package was created"`
	Version                    string            `json:"version" jsonschema:"description=The version of Zarf used to build this package"`
	Migrations                 []string          `json:"migrations,omitempty" jsonschema:"description=Any migrations that have been run on this package"`
	RegistryOverrides          map[string]string `json:"registryOverrides,omitempty" jsonschema:"description=Any registry domains that were overridden on package create when pulling images"`
	Differential               bool              `json:"differential,omitempty" jsonschema:"description=Whether this package was created with differential components"`
	DifferentialPackageVersion string            `json:"differentialPackageVersion,omitempty" jsonschema:"description=Version of a previously built package used as the basis for creating this differential package"`
	DifferentialMissing        []string          `json:"differentialMissing,omitempty" jsonschema:"description=List of components that were not included in this package due to differential packaging"`
	LastNonBreakingVersion     string            `json:"lastNonBreakingVersion,omitempty" jsonschema:"description=The minimum version of Zarf that does not have breaking package structure changes"`
	Flavor                     string            `json:"flavor,omitempty" jsonschema:"description=The flavor of Zarf used to build this package"`
}
