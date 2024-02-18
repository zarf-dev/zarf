// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"github.com/defenseunicorns/zarf/src/types"
)

// ComponentFilterStrategy is a strategy interface for filtering components.
type ComponentFilterStrategy interface {
	Apply(types.ZarfPackage) ([]types.ZarfComponent, error)
}
