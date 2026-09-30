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
	// Apply returns the components to keep, in order.
	Apply(api.Package) ([]api.Component, error)
}

// Apply applies a component filter to a package definition.
func Apply(definition api.Package, filter ComponentFilterStrategy) (api.Package, error) {
	if filter == nil {
		filter = Empty()
	}

	components, err := filter.Apply(definition)
	if err != nil {
		return api.Package{}, err
	}

	definition.Components = components
	return definition, nil
}

// comboFilter is a filter that applies a sequence of filters.
type comboFilter struct {
	filters []ComponentFilterStrategy
}

// Apply applies the filter.
func (f *comboFilter) Apply(pkg api.Package) ([]api.Component, error) {
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
