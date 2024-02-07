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
	LoadPackageDefinition(dst *layout.PackagePaths) (loadedPkg *types.ZarfPackage, warnings []string, err error)
	Assemble(dst *layout.PackagePaths, loadedPkg *types.ZarfPackage) error
	Output(dst *layout.PackagePaths, loadedPkg *types.ZarfPackage) error
}
