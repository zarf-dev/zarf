// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	// veryify that SkeletonCreator implements Creator
	_ Creator = (*SkeletonCreator)(nil)
)

// SkeletonCreator provides methods for creating skeleton Zarf packages.
type SkeletonCreator struct{}

// LoadPackageDefinition loads and configure a zarf.yaml file during package create.
func (sc *SkeletonCreator) LoadPackageDefinition(pkg *types.ZarfPackage, createOpts *types.ZarfCreateOptions, _ *layout.PackagePaths) (loadedPkg *types.ZarfPackage, warnings []string, err error) {
	pkg, err = setPackageMetadata(pkg, createOpts)
	if err != nil {
		message.Warn(err.Error())
	}

	// Compose components into a single zarf.yaml file
	pkg, composeWarnings, err := ComposeComponents(pkg, createOpts)
	if err != nil {
		return nil, nil, err
	}
	warnings = append(warnings, composeWarnings...)

	return pkg, warnings, nil
}
