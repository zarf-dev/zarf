// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1beta1

// These types are duplicated from v1alpha1 and carried as backwards-compatibility
// shims so v1alpha1 packages can be converted to v1beta1 without v1beta1 importing
// v1alpha1. They are not part of the v1beta1 schema.

// VariableType represents a type of a Zarf package variable.
type VariableType string

const (
	// RawVariableType is the default type for a Zarf package variable.
	RawVariableType VariableType = "raw"
	// FileVariableType is a type for a Zarf package variable that loads its contents from a file.
	FileVariableType VariableType = "file"
)

// Variable represents a variable that has a value set programmatically.
type Variable struct {
	// The name to be used for the variable.
	Name string
	// Whether to mark this variable as sensitive to not print it in the log.
	Sensitive bool
	// Whether to automatically indent the variable's value (if multiline) when templating.
	AutoIndent bool
	// An optional regex pattern that a variable value must match before a package deployment can continue.
	Pattern string
	// Changes the handling of a variable to load contents differently (i.e. from a file rather than as a raw variable).
	Type VariableType
}

// InteractiveVariable is a variable that can be used to prompt a user for more information.
type InteractiveVariable struct {
	Variable
	// A description of the variable to be used when prompting the user a value.
	Description string
	// The default value to use for the variable.
	Default string
	// Whether to prompt the user for input for this variable.
	Prompt bool
}

// Constant are constants that can be used to dynamically template K8s resources or run in actions.
type Constant struct {
	// The name to be used for the constant.
	Name string
	// The value to set for the constant during deploy.
	Value string
	// A description of the constant to explain its purpose on package create or deploy confirmation prompts.
	Description string
	// Whether to automatically indent the variable's value (if multiline) when templating.
	AutoIndent bool
	// An optional regex pattern that a constant value must match before a package can be created.
	Pattern string
}

// ZarfChartVariable represents a variable that can be set for a Helm chart overrides.
type ZarfChartVariable struct {
	// The name of the variable.
	Name string
	// A brief description of what the variable controls.
	Description string
	// The path within the Helm chart values where this variable applies.
	Path string
}

// ZarfContainerTarget defines the destination info for a ZarfDataInjection target.
type ZarfContainerTarget struct {
	// The namespace to target for data injection.
	Namespace string
	// The K8s selector to target for data injection.
	Selector string
	// The container name to target for data injection.
	Container string
	// The path within the container to copy the data into.
	Path string
}

// ZarfDataInjection is a data-injection definition.
type ZarfDataInjection struct {
	// Either a path to a local folder/file or a remote URL of a file to inject into the given target pod + container.
	Source string
	// The target pod + container to inject the data into.
	Target ZarfContainerTarget
	// Compress the data before transmitting using gzip.
	Compress bool
}
