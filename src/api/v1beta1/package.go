// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 holds the definition of the v1beta1 Zarf Package. This API is work in progress and not yet used within Zarf.
package v1beta1

// PackageKind is an enum of the different kinds of Zarf packages.
type PackageKind string

const (
	// ZarfPackageConfig is the default kind of Zarf package.
	ZarfPackageConfig PackageKind = "ZarfPackageConfig"
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
	Kind PackageKind `json:"kind" jsonschema:"enum=ZarfPackageConfig"`
	// Package metadata.
	Metadata PackageMetadata `json:"metadata,omitempty"`
	// Zarf-generated package build data.
	Build BuildData `json:"build,omitempty"`
	// List of components to deploy in this package.
	Components []Component `json:"components" jsonschema:"minItems=1"`
	// Values imports Zarf values files for templating and overriding Helm values.
	Values Values `json:"values,omitempty"`
	// Documentation files included in the package.
	Documentation map[string]string `json:"documentation,omitempty"`

	// Variables removed from the v1beta1 schema; kept as a v1alpha1 backwards-compatibility shim.
	variables []InteractiveVariable
	// Constants removed from the v1beta1 schema; kept as a v1alpha1 backwards-compatibility shim.
	constants []Constant
}

// GetDeprecatedVariables returns the v1alpha1 variables carried as a backwards-compatibility shim.
func (pkg Package) GetDeprecatedVariables() []InteractiveVariable {
	return pkg.variables
}

// GetDeprecatedConstants returns the v1alpha1 constants carried as a backwards-compatibility shim.
func (pkg Package) GetDeprecatedConstants() []Constant {
	return pkg.constants
}

// HasImages returns true if one of the components contains an image.
func (pkg Package) HasImages() bool {
	for _, component := range pkg.Components {
		if len(component.Images) > 0 {
			return true
		}
	}
	return false
}

// IsSBOMAble checks if a package has contents that an SBOM can be created on (i.e. images, files, or image archives).
func (pkg Package) IsSBOMAble() bool {
	for _, c := range pkg.Components {
		if len(c.Images) > 0 || len(c.Files) > 0 || len(c.ImageArchives) > 0 {
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
	// yolo removed from the v1beta1 schema; kept as a v1alpha1 backwards-compatibility shim.
	yolo bool
}

// GetDeprecatedYOLO returns the v1alpha1 YOLO field carried as a backwards-compatibility shim.
func (m PackageMetadata) GetDeprecatedYOLO() bool {
	return m.yolo
}

// BuildData is written during package create to track details of the created package.
type BuildData struct {
	// The machine name that created this package.
	Hostname string `json:"hostname,omitempty"`
	// The username who created this package.
	User string `json:"user,omitempty"`
	// The architecture this package was created on.
	Architecture string `json:"architecture"`
	// The timestamp when this package was created.
	Timestamp string `json:"timestamp"`
	// The version of Zarf used to build this package.
	Version string `json:"version"`
	// Any migrations that have been run on this package.
	Migrations []string `json:"migrations,omitempty"`
	// Any registry domains that were overridden on package create when pulling images.
	RegistryOverrides map[string]string `json:"registryOverrides,omitempty"`
	// Whether this package was created with differential components.
	Differential bool `json:"differential,omitempty"`
	// Version of a previously built package used as the basis for creating this differential package.
	DifferentialPackageVersion string `json:"differentialPackageVersion,omitempty"`
	// The flavor of Zarf used to build this package.
	Flavor string `json:"flavor,omitempty"`
	// Whether this package was signed.
	Signed *bool `json:"signed,omitempty"`
	// Requirements for specific Zarf versions needed to deploy this package.
	VersionRequirements []VersionRequirement `json:"versionRequirements,omitempty"`
	// ProvenanceFiles lists files present in the package that are not included in checksums.txt. These are files added after checksum generation (e.g., signature files).
	ProvenanceFiles []string `json:"provenanceFiles,omitempty"`
	// Checksum of a checksums.txt file that contains checksums all the layers within the package.
	AggregateChecksum string `json:"aggregateChecksum,omitempty"`
	// originalAPIVersion records the apiVersion the package was read from before any conversion.
	originalAPIVersion string
}

// GetOriginalAPIVersion returns the apiVersion the package was read from before any conversion.
func (b BuildData) GetOriginalAPIVersion() string {
	return b.originalAPIVersion
}

// SetOriginalAPIVersion records the apiVersion the package was read from before any conversion.
func (b *BuildData) SetOriginalAPIVersion(apiVersion string) {
	b.originalAPIVersion = apiVersion
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
