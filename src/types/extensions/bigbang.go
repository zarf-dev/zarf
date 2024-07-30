// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package extensions contains the types for all official extensions.
package extensions

// BigBang holds the configuration for the Big Bang extension
type BigBang struct {
	// The version of Big Bang to use
	Version string `json:"version"`
	// Override repo to pull Big Bang from instead of Repo One
	Repo string `json:"repo,omitempty"`
	// The list of values files to pass to Big Bang; these will be merged together
	ValuesFiles []string `json:"valuesFiles,omitempty"`
	// Whether to skip deploying flux; Defaults to false
	SkipFlux bool `json:"skipFlux,omitempty"`
	// Optional paths to Flux kustomize strategic merge patch files
	FluxPatchFiles []string `json:"fluxPatchFiles,omitempty"`
}
