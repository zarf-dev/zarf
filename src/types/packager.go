// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// PackagerConfig is the main struct that the packager uses to hold high-level options.
type PackagerConfig struct {
	// CreateOpts tracks the user-defined options used to create the package
	CreateOpts ZarfCreateOptions

	// DeployOpts tracks user-defined values for the active deployment
	DeployOpts ZarfDeployOptions

	// InitOpts tracks user-defined values for the active Zarf initialization.
	InitOpts ZarfInitOptions

	// PublishOpts tracks user-defined options used to publish the package
	PublishOpts ZarfPublishOptions

	// Track if CLI prompts should be generated
	IsInteractive bool

	// Track if the package is an init package
	IsInitConfig bool

	// The package data
	Pkg ZarfPackage

	// The active zarf state
	State ZarfState

	// Variables set by the user
	SetVariableMap map[string]string

	// SBOM file paths in the package
	SBOMViewFiles []string
}
