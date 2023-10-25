// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"github.com/defenseunicorns/zarf/src/extensions/bigbang"
	"github.com/defenseunicorns/zarf/src/types"
)

func composeExtensions(c *types.ZarfComponent, override types.ZarfComponent, relativeTo string) {
	// fix the file paths
	if override.Extensions.BigBang != nil {
		component := bigbang.Compose(relativeTo, override)
		c = &component
	}

	// perform any overrides
	if override.Extensions.BigBang != nil {
		if c.Extensions.BigBang == nil {
			c.Extensions.BigBang = override.Extensions.BigBang
		} else {
			if override.Extensions.BigBang.ValuesFiles != nil {
				c.Extensions.BigBang.ValuesFiles = append(c.Extensions.BigBang.ValuesFiles, override.Extensions.BigBang.ValuesFiles...)
			}
			if override.Extensions.BigBang.FluxPatchFiles != nil {
				c.Extensions.BigBang.FluxPatchFiles = append(c.Extensions.BigBang.FluxPatchFiles, override.Extensions.BigBang.FluxPatchFiles...)
			}
		}
	}
}
