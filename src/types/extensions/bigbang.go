// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package extensions contains the types for all official extensions.
package extensions

// BigBang defines a file to deploy.
type BigBang struct {
	Version     string   `json:"version" jsonschema:"description=The version of Big Bang to use"`
	Repo        string   `json:"repo,omitempty" jsonschema:"description=Override repo to pull Big Bang from instead of Repo One"`
	ValuesFiles []string `json:"valuesFiles,omitempty" jsonschema:"description=The list of values files to pass to Big Bang; these will be merged together"`
	SkipFlux    bool     `json:"skipFlux,omitempty" jsonschema:"description=Whether to skip deploying flux; Defaults to false"`
}
