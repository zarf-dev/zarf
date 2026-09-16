// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/api"
)

// ComponentFilterStrategy is a strategy interface for filtering components.
type ComponentFilterStrategy interface {
	// Apply returns the indices of the components to keep, in order.
	Apply(api.Package) ([]int, error)
}

// Apply applies a component filter to a package definition.
func Apply(definition api.Package, filter ComponentFilterStrategy) (api.Package, error) {
	if filter == nil {
		filter = Empty()
	}

	indices, err := filter.Apply(definition)
	if err != nil {
		return api.Package{}, err
	}

	if err := definition.RetainComponents(indices); err != nil {
		return api.Package{}, fmt.Errorf("filter returned invalid component index: %w", err)
	}
	return definition, nil
}

// comboFilter is a filter that applies a sequence of filters.
type comboFilter struct {
	filters []ComponentFilterStrategy
}

// Apply applies the filter.
func (f *comboFilter) Apply(pkg api.Package) ([]int, error) {
	result := pkg
	resultIndices := make([]int, len(pkg.Components))
	for idx := range pkg.Components {
		resultIndices[idx] = idx
	}

	for _, filter := range f.filters {
		indices, err := filter.Apply(result)
		if err != nil {
			return nil, fmt.Errorf("error applying filter %T: %w", filter, err)
		}

		nextIndices := make([]int, 0, len(indices))
		for _, idx := range indices {
			if idx < 0 || idx >= len(result.Components) {
				return nil, fmt.Errorf("error applying filter %T: index %d out of range", filter, idx)
			}
			nextIndices = append(nextIndices, resultIndices[idx])
		}
		if err := result.RetainComponents(indices); err != nil {
			return nil, fmt.Errorf("error applying filter %T: %w", filter, err)
		}
		resultIndices = nextIndices
	}

	return resultIndices, nil
}

// Combine creates a new filter that applies a sequence of filters.
func Combine(filters ...ComponentFilterStrategy) ComponentFilterStrategy {
	return &comboFilter{filters}
}
