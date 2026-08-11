// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
)

// ComponentView is the stable projection a filter sees.
type ComponentView struct {
	Name        string
	Description string
	Optional    bool
	Default     bool
	Group       string
	OnlyLocalOS string

	// Definition is the complete versioned component definition for interactive display.
	Definition any
}

// PackageView is the stable package projection a filter sees.
type PackageView struct {
	Components []ComponentView
}

// ComponentFilterStrategy is a strategy interface for filtering components.
type ComponentFilterStrategy interface {
	// Apply returns the indices of the components to keep, in order.
	Apply(PackageView) ([]int, error)
}

// comboFilter is a filter that applies a sequence of filters.
type comboFilter struct {
	filters []ComponentFilterStrategy
}

// Apply applies the filter.
func (f *comboFilter) Apply(pkg PackageView) ([]int, error) {
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

		components := make([]ComponentView, 0, len(indices))
		nextIndices := make([]int, 0, len(indices))
		for _, idx := range indices {
			if idx < 0 || idx >= len(result.Components) {
				return nil, fmt.Errorf("error applying filter %T: index %d out of range", filter, idx)
			}
			components = append(components, result.Components[idx])
			nextIndices = append(nextIndices, resultIndices[idx])
		}
		result.Components = components
		resultIndices = nextIndices
	}

	return resultIndices, nil
}

// Combine creates a new filter that applies a sequence of filters.
func Combine(filters ...ComponentFilterStrategy) ComponentFilterStrategy {
	return &comboFilter{filters}
}
