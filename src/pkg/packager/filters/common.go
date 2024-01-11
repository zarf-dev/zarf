// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import "github.com/defenseunicorns/zarf/src/types"

type ComponentFilterStrategy interface {
	Apply([]types.ZarfComponent) ([]types.ZarfComponent, error)
}
