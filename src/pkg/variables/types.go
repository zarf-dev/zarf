// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for interacting with variables
package variables

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
