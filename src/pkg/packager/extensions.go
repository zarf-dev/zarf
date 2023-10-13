// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/extensions/bigbang"
	"github.com/defenseunicorns/zarf/src/types"
)

// Check for any extensions in use and runs the appropriate functions.
func (p *Packager) processExtensions() (err error) {
	components := []types.ZarfComponent{}

	// Create component paths and process extensions for each component.
	for _, c := range p.cfg.Pkg.Components {
		componentPaths, err := p.layout.Components.Create(c)
		if err != nil {
			return err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if c, err = bigbang.Run(p.cfg.Pkg.Metadata.YOLO, componentPaths, c); err != nil {
				return fmt.Errorf("unable to process bigbang extension: %w", err)
			}
		}

		components = append(components, c)
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	p.cfg.Pkg.Components = components

	return nil
}

// Mutate any local files to be relative to the parent
func (p *Packager) composeExtensions(pathAncestry string, component types.ZarfComponent) types.ZarfComponent {
	// Big Bang
	if component.Extensions.BigBang != nil {
		component = bigbang.Compose(pathAncestry, component)
	}

	return component
}

// Check for any extensions in use and skeletonize their local files.
func (p *Packager) skeletonizeExtensions() (err error) {
	components := []types.ZarfComponent{}

	// Create component paths and process extensions for each component.
	for _, c := range p.cfg.Pkg.Components {
		componentPaths, err := p.layout.Components.Create(c)
		if err != nil {
			return err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if c, err = bigbang.Skeletonize(componentPaths, c); err != nil {
				return fmt.Errorf("unable to process bigbang extension: %w", err)
			}
		}

		components = append(components, c)
	}

	// Update the parent package config with the expanded sub components.
	// This is important when the deploy package is created.
	p.cfg.Pkg.Components = components

	return nil
}
