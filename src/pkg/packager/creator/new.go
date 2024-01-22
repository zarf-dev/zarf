// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/types"
)

// Creator is an interface for creating Zarf packages.
type Creator interface {
	LoadPackageDefinition(*types.ZarfPackage, *types.ZarfCreateOptions, *layout.PackagePaths) (*types.ZarfPackage, []string, error)
}

// New returns a new Creator based on the provided create options.
func New(createOpts types.ZarfCreateOptions) Creator {
	if createOpts.IsSkeleton {
		return &SkeletonCreator{}
	}
	return &PackageCreator{}
}
