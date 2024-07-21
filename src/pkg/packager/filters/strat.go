// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/types"
)

// ComponentFilterStrategy is a strategy interface for filtering components.
type ComponentFilterStrategy interface {
	Apply(types.ZarfPackage) ([]types.ZarfComponent, error)
}

// comboFilter is a filter that applies a sequence of filters.
type comboFilter struct {
	filters []ComponentFilterStrategy
}

// Apply applies the filter.
func (f *comboFilter) Apply(pkg types.ZarfPackage) ([]types.ZarfComponent, error) {
	result := pkg

	for _, filter := range f.filters {
		components, err := filter.Apply(result)
		if err != nil {
			return nil, fmt.Errorf("error applying filter %T: %w", filter, err)
		}
		result.Components = components
	}

	return result.Components, nil
}

// Combine creates a new filter that applies a sequence of filters.
func Combine(filters ...ComponentFilterStrategy) ComponentFilterStrategy {
	return &comboFilter{filters}
}
