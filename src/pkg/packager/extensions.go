// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/extensions/bigbang"
)

// Check for any extensions in use and runs the appropriate functions.
func (p *Packager) processExtensions() error {
	// Create component paths and process extensions for each component.
	for i, c := range p.cfg.Pkg.Components {
		componentPath, err := p.createOrGetComponentPaths(c)
		if err != nil {
			return err
		}

		// Big Bang
		if c.Extensions.BigBang != nil {
			if p.cfg.Pkg.Components[i], err = bigbang.Run(p.cfg.Pkg.Metadata.YOLO, componentPath, c); err != nil {
				return fmt.Errorf("unable to process bigbang extension: %s", err.Error())
			}
		}
	}

	return nil
}
