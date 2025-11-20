// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package schema provides functions for generating and working with the Zarf JSON schema.
package schema

//go:generate go run generate.go

import (
	_ "embed"
)

//go:embed zarf-v1alpha1-schema.json
var v1Alpha1Schema []byte

// GetV1Alpha1Schema returns the embedded JSON schema for the v1alpha1 Zarf package config
func GetV1Alpha1Schema() []byte {
	return v1Alpha1Schema
}
