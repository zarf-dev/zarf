// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import "github.com/defenseunicorns/zarf/src/types"

var (
	// verify that PackageCreator implements Creator
	_ Creator = (*PackageCreator)(nil)

	// verify that SkeletonCreator implements Creator
	_ Creator = (*SkeletonCreator)(nil)
)

// Creator is an interface for creating Zarf packages.
type Creator interface {
	CdToBaseDir(createOpts *types.ZarfCreateOptions, cwd string) error
	LoadPackageDefinition(*Packager) error
	Assemble(*Packager) error
}

// NewCreator returns a new Creator based on the provided create options.
func NewCreator(createOpts *types.ZarfCreateOptions) Creator {
	if createOpts.IsSkeleton {
		return &SkeletonCreator{}
	}
	return &PackageCreator{}
}

// PackageCreator is used to create normal (not skeleton) Zarf packages.
type PackageCreator struct{}

// SkeletonCreator is used to create skeleton Zarf packages.
type SkeletonCreator struct{}
