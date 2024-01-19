// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package creator contains functions for creating Zarf packages.
package creator

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/extensions/bigbang"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/types"
)

func ProcessExtensions(pkg *types.ZarfPackage, createOpts *types.ZarfCreateOptions, layout *layout.PackagePaths) (*types.ZarfPackage, error) {
	components := []types.ZarfComponent{}

	// Create component paths and process extensions for each component.
	for _, c := range pkg.Components {
		componentPaths, err := layout.Components.Create(c)
		if err != nil {
			return nil, err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if createOpts.IsSkeleton {
				if c, err = bigbang.Skeletonize(componentPaths, c); err != nil {
					return nil, fmt.Errorf("unable to process bigbang extension: %w", err)
				}
			}
			if c, err = bigbang.Run(pkg.Metadata.YOLO, componentPaths, c); err != nil {
				return nil, fmt.Errorf("unable to process bigbang extension: %w", err)
			}
		}

		components = append(components, c)
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	pkg.Components = components

	return pkg, nil
}
