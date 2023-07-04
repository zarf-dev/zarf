// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

// BundlerConfig is the main struct that the bundler uses to hold high-level options.
type BundlerConfig struct {
	CreateOpts  BundlerCreateOptions
	DeployOpts  BundlerDeployOptions
	PullOpts    BundlerPullOptions
	InspectOpts BundlerInspectOptions
	RemoveOpts  BundlerRemoveOptions
	State       ZarfState

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
	Packages     []string
	Source       string
	SetVariables map[string]string
}

// BundlerInspectOptions is the options for the bundler.Inspect() function
type BundlerInspectOptions struct {
	PublicKey string
	Source    string
}

// BundlerPullOptions is the options for the bundler.Pull() function
type BundlerPullOptions struct {
	OutputDirectory string
	PublicKey       string
	Source          string
}

// BundlerRemoveOptions is the options for the bundler.Remove() function
type BundlerRemoveOptions struct {
	Packages []string
	Source   string
}
