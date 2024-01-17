// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import "github.com/defenseunicorns/zarf/src/types"

// IsInitConfig returns whether the provided Zarf package is an init config.
func IsInitConfig(pkg types.ZarfPackage) bool {
	return pkg.Kind == types.ZarfInitConfig
}
