// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// BundlerConfig is the main struct that the bundler uses to hold high-level options.
type BundlerConfig struct {
	// CreateOpts tracks the user-defined options used to create the package
	CreateOpts ZarfCreateOptions

	// DeployOpts tracks user-defined values for the active deployment
	// for a bundle, this is a combination of the deploy and init options
	// since a bundle can contain both.
	DeployOpts BundlerDeployOptions

	// PullOpts tracks user-defined options used to pull packages
	PullOpts ZarfPullOptions

	// Track if CLI prompts should be generated
	IsInteractive bool

	// The bundle data
	Bndl ZarfBundle

	// The original source of the bundle
	BndlSource string

	// The active zarf state
	State ZarfState

	// Variables set by the user
	SetVariableMap map[string]*ZarfSetVariable

	// SBOM file paths in the bundle
	SBOMViewFiles []string
}

type BundlerDeployOptions struct {
	ZarfDeployOptions
	ZarfInitOptions
}
