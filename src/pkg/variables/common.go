// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for interacting with variables
package variables

import (
	"log/slog"
)

// VariableConfig represents a value to be templated into a text file.
type VariableConfig struct {
	templatePrefix string
	deprecatedKeys map[string]string

	applicationTemplates map[string]*TextTemplate
	setVariableMap       SetVariableMap
	constants            []Constant

	prompt func(variable InteractiveVariable) (value string, err error)
	logger *slog.Logger
}

// New creates a new VariableConfig
func New(templatePrefix string, deprecatedKeys map[string]string, prompt func(variable InteractiveVariable) (value string, err error), logger *slog.Logger) *VariableConfig {
	return &VariableConfig{
		templatePrefix:       templatePrefix,
		deprecatedKeys:       deprecatedKeys,
		applicationTemplates: make(map[string]*TextTemplate),
		setVariableMap:       make(SetVariableMap),
		prompt:               prompt,
		logger:               logger,
	}
}

func (vc *VariableConfig) SetApplicationTemplates(applicationTemplates map[string]*TextTemplate) {
	vc.applicationTemplates = applicationTemplates
}

func (vc *VariableConfig) SetConstants(constants []Constant) {
	vc.constants = constants
}
