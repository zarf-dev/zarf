// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package schema provides functions for generating and working with the Zarf JSON schema.
package schema

//go:generate go run generate.go

import (
	_ "embed"
)

//go:embed zarf-schema.json
var schema []byte

// GetV1Alpha1Schema returns the embedded JSON schema for the Zarf package configuration.
// The schema is generated at compile time via go:generate.
func GetV1Alpha1Schema() []byte {
	return schema
}
