// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// verify that SkeletonCreator implements Creator
	_ Creator = (*SkeletonCreator)(nil)
)

// SkeletonCreator provides methods for creating skeleton Zarf packages.
type SkeletonCreator struct{}

// CdToBaseDir changes the current working directory to the specified base directory.
func (p *SkeletonCreator) CdToBaseDir(createOpts *types.ZarfCreateOptions, cwd string) error {
	return cdToBaseDir(createOpts, cwd)
}
