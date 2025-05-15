// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1alpha1 holds the definition of the v1alpha1 Zarf Package
package v1alpha1

import (
	"fmt"
	"regexp"
)

// VariableType represents a type of a Zarf package variable
type VariableType string

const (
	// RawVariableType is the default type for a Zarf package variable
	RawVariableType VariableType = "raw"
	// FileVariableType is a type for a Zarf package variable that loads its contents from a file
	FileVariableType VariableType = "file"
)

// IsUppercaseNumberUnderscore is a regex for uppercase, numbers and underscores.
// https://regex101.com/r/tfsEuZ/1
var IsUppercaseNumberUnderscore = regexp.MustCompile(`^[A-Z0-9_]+$`).MatchString

// Zarf looks for these strings in zarf.yaml to make dynamic changes
const (
	ZarfPackageTemplatePrefix = "###ZARF_PKG_TMPL_"
	ZarfPackageVariablePrefix = "###ZARF_PKG_VAR_"
	ZarfPackageArch           = "###ZARF_PKG_ARCH###"
	ZarfComponentName         = "###ZARF_COMPONENT_NAME###"
)

// ZarfPackageKind is an enum of the different kinds of Zarf packages.
type ZarfPackageKind string

const (
	// ZarfInitConfig is the kind of Zarf package used during `zarf init`.
	ZarfInitConfig ZarfPackageKind = "ZarfInitConfig"
	// ZarfPackageConfig is the default kind of Zarf package, primarily used during `zarf package`.
	ZarfPackageConfig ZarfPackageKind = "ZarfPackageConfig"
	// APIVersion the api version of this package.
	APIVersion string = "zarf.dev/v1alpha1"
)

// ZarfPackage the top-level structure of a Zarf config file.
type ZarfPackage struct {
	// The API version of the Zarf package.
	APIVersion string `json:"apiVersion,omitempty," jsonschema:"enum=zarf.dev/v1alpha1"`
	// The kind of Zarf package.
	Kind ZarfPackageKind `json:"kind" jsonschema:"enum=ZarfInitConfig,enum=ZarfPackageConfig,default=ZarfPackageConfig"`
	// Package metadata.
	Metadata ZarfMetadata `json:"metadata,omitempty"`
	// Zarf-generated package build data.
	Build ZarfBuildData `json:"build,omitempty"`
	// List of components to deploy in this package.
	Components []ZarfComponent `json:"components" jsonschema:"minItems=1"`
	// Constant template values applied on deploy for K8s resources.
	Constants []Constant `json:"constants,omitempty"`
	// Variable template values applied on deploy for K8s resources.
	Variables []InteractiveVariable `json:"variables,omitempty"`
}

// IsInitConfig returns whether a Zarf package is an init config.
func (pkg ZarfPackage) IsInitConfig() bool {
	return pkg.Kind == ZarfInitConfig
}

// HasImages returns true if one of the components contains an image.
func (pkg ZarfPackage) HasImages() bool {
	for _, component := range pkg.Components {
		if len(component.Images) > 0 {
			return true
		}
	}
	return false
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

// Variable represents a variable that has a value set programmatically
type Variable struct {
	// The name to be used for the variable
	Name string `json:"name" jsonschema:"pattern=^[A-Z0-9_]+$"`
	// Whether to mark this variable as sensitive to not print it in the log
	Sensitive bool `json:"sensitive,omitempty"`
	// Whether to automatically indent the variable's value (if multiline) when templating. Based on the number of chars before the start of ###ZARF_VAR_.
	AutoIndent bool `json:"autoIndent,omitempty"`
	// An optional regex pattern that a variable value must match before a package deployment can continue.
	Pattern string `json:"pattern,omitempty"`
	// Changes the handling of a variable to load contents differently (i.e. from a file rather than as a raw variable - templated files should be kept below 1 MiB)
	Type VariableType `json:"type,omitempty" jsonschema:"enum=raw,enum=file"`
}

// InteractiveVariable is a variable that can be used to prompt a user for more information
type InteractiveVariable struct {
	Variable `json:",inline"`
	// A description of the variable to be used when prompting the user a value
	Description string `json:"description,omitempty"`
	// The default value to use for the variable
	Default string `json:"default,omitempty"`
	// Whether to prompt the user for input for this variable
	Prompt bool `json:"prompt,omitempty"`
}

// Constant are constants that can be used to dynamically template K8s resources or run in actions.
type Constant struct {
	// The name to be used for the constant
	Name string `json:"name" jsonschema:"pattern=^[A-Z0-9_]+$"`
	// The value to set for the constant during deploy
	Value string `json:"value"`
	// A description of the constant to explain its purpose on package create or deploy confirmation prompts
	Description string `json:"description,omitempty"`
	// Whether to automatically indent the variable's value (if multiline) when templating. Based on the number of chars before the start of ###ZARF_CONST_.
	AutoIndent bool `json:"autoIndent,omitempty"`
	// An optional regex pattern that a constant value must match before a package can be created.
	Pattern string `json:"pattern,omitempty"`
}

// SetVariable tracks internal variables that have been set during this run of Zarf
type SetVariable struct {
	Variable `json:",inline"`
	// The value the variable is currently set with
	Value string `json:"value"`
}

// Validate runs all validation checks on a package constant.
func (c Constant) Validate() error {
	if !regexp.MustCompile(c.Pattern).MatchString(c.Value) {
		return fmt.Errorf("provided value for constant %s does not match pattern %s", c.Name, c.Pattern)
	}
	return nil
}

// ZarfMetadata lists information about the current ZarfPackage.
type ZarfMetadata struct {
	// Name to identify this Zarf package.
	Name string `json:"name" jsonschema:"pattern=^[a-z0-9][a-z0-9\\-]*$"`
	// Additional information about this package.
	Description string `json:"description,omitempty"`
	// Generic string set by a package author to track the package version (Note: ZarfInitConfigs will always be versioned to the CLIVersion they were created with).
	Version string `json:"version,omitempty"`
	// Link to package information when online.
	URL string `json:"url,omitempty"`
	// An image URL to embed in this package (Reserved for future use in Zarf UI).
	Image string `json:"image,omitempty"`
	// Disable compression of this package.
	Uncompressed bool `json:"uncompressed,omitempty"`
	// The target cluster architecture for this package.
	Architecture string `json:"architecture,omitempty" jsonschema:"example=arm64,example=amd64"`
	// Yaml OnLy Online (YOLO): True enables deploying a Zarf package without first running zarf init against the cluster. This is ideal for connected environments where you want to use existing VCS and container registries.
	YOLO bool `json:"yolo,omitempty"`
	// Comma-separated list of package authors (including contact info).
	Authors string `json:"authors,omitempty" jsonschema:"example=Doug &#60;hello@defenseunicorns.com&#62;&#44; Pepr &#60;hello@defenseunicorns.com&#62;"`
	// Link to package documentation when online.
	Documentation string `json:"documentation,omitempty"`
	// Link to package source code when online.
	Source string `json:"source,omitempty"`
	// Name of the distributing entity, organization or individual.
	Vendor string `json:"vendor,omitempty"`
	// Checksum of a checksums.txt file that contains checksums all the layers within the package.
	AggregateChecksum string `json:"aggregateChecksum,omitempty"`
	// Annotations contains arbitrary metadata about the package.
	// Users are encouraged to follow OCI image-spec https://github.com/opencontainers/image-spec/blob/main/annotations.md
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ZarfBuildData is written during the packager.Create() operation to track details of the created package.
type ZarfBuildData struct {
	// The machine name that created this package.
	Terminal string `json:"terminal"`
	// The username who created this package.
	User string `json:"user"`
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
	// List of components that were not included in this package due to differential packaging.
	DifferentialMissing []string `json:"differentialMissing,omitempty"`
	// The minimum version of Zarf that does not have breaking package structure changes.
	LastNonBreakingVersion string `json:"lastNonBreakingVersion,omitempty"`
	// The flavor of Zarf used to build this package.
	Flavor string `json:"flavor,omitempty"`
}
