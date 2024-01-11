// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// verify that PackageCreator implements Creator
	_ Creator = (*PackageCreator)(nil)
)

// PackageCreator provides methods for creating normal (not skeleton) Zarf packages.
type PackageCreator struct{}

// CdToBaseDir changes the current working directory to the specified base directory.
func (p *PackageCreator) CdToBaseDir(createOpts *types.ZarfCreateOptions, cwd string) error {
	return cdToBaseDir(createOpts, cwd)
}
