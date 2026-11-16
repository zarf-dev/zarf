// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package variables contains functions for interacting with variables
//
// Deprecated: Only supported by deprecated v1alpha1 packages.
package variables

import (
	"log/slog"

	"github.com/zarf-dev/zarf/src/api"
)

// VariableConfig represents a value to be templated into a text file.
type VariableConfig struct {
	templatePrefix string

	applicationTemplates map[string]*TextTemplate
	setVariableMap       SetVariableMap
	constants            []api.Constant

	prompt func(variable api.InteractiveVariable) (value string, err error)
	logger *slog.Logger
}

// New creates a new VariableConfig
//
// Deprecated: Only supported by deprecated v1alpha1 packages.
func New(templatePrefix string, prompt func(variable api.InteractiveVariable) (value string, err error), logger *slog.Logger) *VariableConfig {
	return &VariableConfig{
		templatePrefix:       templatePrefix,
		applicationTemplates: make(map[string]*TextTemplate),
		setVariableMap:       make(SetVariableMap),
		prompt:               prompt,
		logger:               logger,
	}
}

// SetApplicationTemplates sets the application-specific templates for the variable config (i.e. ZARF_REGISTRY for Zarf)
//
// Deprecated: Only supported by deprecated v1alpha1 packages.
func (vc *VariableConfig) SetApplicationTemplates(applicationTemplates map[string]*TextTemplate) {
	vc.applicationTemplates = applicationTemplates
}

// SetConstants sets the constants for a variable config (templated as PREFIX_CONST_NAME)
//
// Deprecated: Only supported by deprecated v1alpha1 packages.
func (vc *VariableConfig) SetConstants(constants []api.Constant) {
	vc.constants = constants
}

// GetConstants fetches the package constants.
//
// Deprecated: Only supported by deprecated v1alpha1 packages.
func (vc *VariableConfig) GetConstants() []api.Constant {
	return vc.constants
}
