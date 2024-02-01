// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for interacting with variables
package variables

// VariableConfig represents a value to be templated into a text file.
type VariableConfig struct {
	templatePrefix string
	deprecatedKeys map[string]string

	ApplicationTemplates map[string]*TextTemplate
	SetVariableMap       SetVariableMap
	Constants            []Constant

	logger func(format string, a ...any)
}

func New(templatePrefix string, deprecatedKeys map[string]string, setVariableMap SetVariableMap, constants []Constant, logger func(format string, a ...any)) *VariableConfig {
	return &VariableConfig{
		templatePrefix:       templatePrefix,
		deprecatedKeys:       deprecatedKeys,
		ApplicationTemplates: make(map[string]*TextTemplate),
		SetVariableMap:       setVariableMap,
		Constants:            constants,
		logger:               logger,
	}
}
