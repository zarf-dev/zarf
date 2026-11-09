// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 holds the definition of the v1beta1 Zarf Package.
package v1beta1

import "fmt"

// PackageKind is an enum of the different kinds of Zarf packages.
type PackageKind string

const (
	// ZarfPackageConfig is the default kind of Zarf package.
	ZarfPackageConfig PackageKind = "ZarfPackageConfig"
	// ZarfInitConfig is the kind of package used during zarf init.
	ZarfInitConfig PackageKind = "ZarfInitConfig"
	// ZarfComponentConfig is the kind of a Zarf component config file.
	ZarfComponentConfig PackageKind = "ZarfComponentConfig"
	// APIVersion is the api version of this package.
	APIVersion string = "zarf.dev/v1beta1"
)

// Package is the top-level structure of a Zarf package definition.
type Package struct {
	// The API version of the Zarf package.
	APIVersion string `json:"apiVersion" jsonschema:"enum=zarf.dev/v1beta1"`
	// The kind of Zarf package.
	Kind PackageKind `json:"kind" jsonschema:"enum=ZarfInitConfig,enum=ZarfPackageConfig,default=ZarfPackageConfig"`
	// Package metadata.
	Metadata PackageMetadata `json:"metadata,omitempty"`
	// Zarf-generated package build data.
	Build BuildData `json:"build,omitempty"`
	// List of components to deploy in this package.
	Components []Component `json:"components" jsonschema:"minItems=1"`
	// Values imports Zarf values files for templating and overriding Helm values.
	Values Values `json:"values,omitempty"`
	// Attachments are files included in the package.
	Attachments map[string]string `json:"attachments,omitempty"`
}

// GetComponent returns the component with the given name, or an error if no such component exists.
func (pkg Package) GetComponent(name string) (Component, error) {
	for _, component := range pkg.Components {
		if component.Name == name {
			return component, nil
		}
	}
	return Component{}, fmt.Errorf("no component named %q in package %q", name, pkg.Metadata.Name)
}

// HasImages returns true if one of the components contains an image or image archive.
func (pkg Package) HasImages() bool {
	for _, component := range pkg.Components {
		if len(component.Images) > 0 || len(component.ImageArchives) > 0 {
			return true
		}
	}
	return false
}

// PackageMetadata holds information about the package.
type PackageMetadata struct {
	// Name to identify this Zarf package.
	Name string `json:"name" jsonschema:"pattern=^[a-z0-9][a-z0-9\\-]*$"`
	// Additional information about this Zarf package.
	Description string `json:"description,omitempty"`
	// Generic string set by a package author to track the package version.
	Version string `json:"version,omitempty"`
	// Disable compression of this package.
	Uncompressed bool `json:"uncompressed,omitempty"`
	// The target cluster architecture for this package.
	Architecture string `json:"architecture,omitempty" jsonschema:"example=arm64,example=amd64"`
	// Annotations are key-value pairs that can be used to store metadata about the package.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Prevent namespace overrides for this package.
	PreventNamespaceOverride bool `json:"preventNamespaceOverride,omitempty"`
}

// BuildData is written during package create to track details of the created package.
type BuildData struct {
	// The machine name that created this package.
	Hostname string `json:"hostname,omitempty"`
	// The username who created this package.
	User string `json:"user,omitempty"`
	// The timestamp when this package was created.
	Timestamp string `json:"timestamp,omitempty"`
	// The architecture this package was created on.
	Architecture string `json:"architecture"`
	// The version of Zarf used to build this package.
	Version string `json:"version"`
	// Any migrations that have been run on this package.
	Migrations []string `json:"migrations,omitempty"`
	// Any registry domains that were overridden on package create when pulling images.
	RegistryOverrides map[string]string `json:"registryOverrides,omitempty"`
	// Base package used to create this differential package. Omitted for full packages.
	Differential *Differential `json:"differential,omitempty"`
	// The flavor of Zarf used to build this package.
	Flavor string `json:"flavor,omitempty"`
	// Whether this package was signed.
	Signed *bool `json:"signed,omitempty"`
	// Requirements for specific Zarf versions needed to deploy this package.
	VersionRequirements []VersionRequirement `json:"versionRequirements,omitempty"`
	// ProvenanceFiles lists files present in the package that are not included in checksums.txt. These are files added after checksum generation (e.g., signature files).
	ProvenanceFiles []string `json:"provenanceFiles,omitempty"`
	// Checksum of a checksums.txt file that contains checksums all the layers within the package.
	AggregateChecksum string `json:"aggregateChecksum"`
}

// Differential identifies the base package used to create a differential package.
type Differential struct {
	// Version of a previously built package used as the basis for creating this differential package.
	FromVersion string `json:"fromVersion"`
}

// VersionRequirement specifies a minimum Zarf version needed and the reason for the requirement.
type VersionRequirement struct {
	// The minimum version of Zarf required.
	Version string `json:"version"`
	// The reason this version is required.
	Reason string `json:"reason"`
}

// Values defines values files and schema for templating and overriding Helm values.
type Values struct {
	// List of values file paths to include.
	Files []string `json:"files,omitempty"`
	// Path to a JSON schema file for validating values.
	Schema string `json:"schema,omitempty"`
}
