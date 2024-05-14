// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/packager/lint"
	"github.com/defenseunicorns/zarf/src/types"
)

// Creator is an interface for creating Zarf packages.
type Creator interface {
	LoadPackageDefinition(dst *layout.PackagePaths) (pkg types.ZarfPackage, findings []lint.ValidatorMessage, err error)
	Assemble(dst *layout.PackagePaths, components []types.ZarfComponent, arch string) error
	Output(dst *layout.PackagePaths, pkg *types.ZarfPackage) error
}
