// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package extensions contains the types for all official extensions.
package extensions

// BigBang defines a file to deploy.
type BigBang struct {
	Version    string   `json:"version" jsonschema:"description=The version of Big Bang you'd like to use"`
	Repo       string   `json:"repo,omitempty" jsonschema:"description=Override of repo to pull big bang from"`
	ValuesFrom []string `json:"valuesFrom,omitempty" jsonschema:"description=list of values files to pass to BigBang; these will be merged together"`
	SkipFlux   bool     `json:"skipFlux,omitempty" jsonschema:"description=Should we skip deploying flux? Defaults to false"`
}
