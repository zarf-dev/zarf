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
		component := bigbang.Compose(*c, override, relativeTo)
		c = &component
	}
}
