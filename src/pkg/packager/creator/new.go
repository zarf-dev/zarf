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
	LoadPackageDefinition() (*types.ZarfPackage, []string, error)
	Assemble() error
	Output() error
}

// New returns a new Creator based on the provided create options.
func New(cfg *types.PackagerConfig, layout *layout.PackagePaths) Creator {
	if cfg.CreateOpts.IsSkeleton {
		return &SkeletonCreator{cfg, layout}
	}
	return &PackageCreator{cfg, layout}
}
