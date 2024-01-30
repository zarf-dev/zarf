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
	LoadPackageDefinition(dst *layout.PackagePaths) (*types.ZarfPackage, []string, error)
	Assemble(dst *layout.PackagePaths) error
	Output(dst *layout.PackagePaths) error
}

// New returns a new Creator based on the provided create options.
func New(cfg *types.PackagerConfig) Creator {
	if cfg.CreateOpts.IsSkeleton {
		return &SkeletonCreator{cfg}
	}
	return &PackageCreator{cfg}
}
