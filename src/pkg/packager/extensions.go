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
func (p *Packager) processExtensions(cPaths types.ComponentPaths, c types.ZarfComponent) (types.ZarfComponent, error) {
	var err error

	// Big Bang
	if c.Extensions.BigBang != nil {
		if c, err = bigbang.Run(cPaths, c); err != nil {
			return c, fmt.Errorf("unable to process bigbang extension: %w", err)
		}
	}

	return c, nil
}
