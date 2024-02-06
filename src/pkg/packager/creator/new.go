// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/types"
)

// Creator is an interface for creating Zarf packages.
type Creator interface {
	LoadPackageDefinition(dst *layout.PackagePaths) (loadedPkg *types.ZarfPackage, warnings []string, err error)
	Assemble(dst *layout.PackagePaths, loadedPkg *types.ZarfPackage) error
	Output(dst *layout.PackagePaths, loadedPkg *types.ZarfPackage) error
}

// New returns a new Creator based on the provided create options.
func New(createOpts types.ZarfCreateOptions, cwd string) Creator {
	if createOpts.IsSkeleton {
		return &skeletonCreator{createOpts: createOpts}
	}

	// differentials are relative to the current working directory
	if createOpts.DifferentialData.DifferentialPackagePath != "" {
		createOpts.DifferentialData.DifferentialPackagePath = filepath.Join(cwd, createOpts.DifferentialData.DifferentialPackagePath)
	}

	return &packageCreator{createOpts: createOpts}
}
