// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// BundlerConfig is the main struct that the bundler uses to hold high-level options.
type BundlerConfig struct {
	// CreateOpts tracks the user-defined options used to create the package
	CreateOpts BundlerCreateOptions

	// DeployOpts tracks user-defined values for the active deployment
	DeployOpts BundlerDeployOptions

	// PullOpts tracks user-defined options used to pull packages
	PullOpts BundlerPullOptions

	InspectOpts BundlerInspectOptions

	RemoveOpts BundlerRemoveOptions

	// The bundle data
	Bndl ZarfBundle

	// The active zarf state
	State ZarfState

	// Variables set by the user
	SetVariableMap map[string]*ZarfSetVariable
}

// BundlerCreateOptions is the options for the bundler.Create() function
type BundlerCreateOptions struct {
	SourceDirectory    string
	Output             string
	SigningKeyPath     string
	SigningKeyPassword string
	SetVariables       map[string]string
}

// BundlerDeployOptions is the options for the bundler.Deploy() function
type BundlerDeployOptions struct {
	Source       string
	SetVariables map[string]string
}

// BundlerInspectOptions is the options for the bundler.Inspect() function
type BundlerInspectOptions struct {
	Source string
}

// BundlerPullOptions is the options for the bundler.Pull() function
type BundlerPullOptions struct {
	Source          string
	OutputDirectory string
	Packages        []string
}

// BundlerRemoveOptions is the options for the bundler.Remove() function
type BundlerRemoveOptions struct {
	Source string
}
