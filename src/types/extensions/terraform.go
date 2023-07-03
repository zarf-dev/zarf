// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package extensions contains the types for all official extensions.
package extensions

// Terraform defines a set of Terraform to deploy.
type Terraform struct {
	Source  string `json:"source,omitempty" jsonschema:"description=The source directory containing your Terraform files"`
	Version string `json:"version" jsonschema:"description=The version of Terraform to install (if specified)"`
}
