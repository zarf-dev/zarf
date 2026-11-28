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
	// Use the package-level Apply function to retain deployment selection provenance.
	Apply(api.Package) ([]api.Component, error)
}

// Package-aware strategies preserve runtime provenance without changing the
// public component-only strategy contract used by custom filters.
type packageFilterStrategy interface {
	applyPackage(api.Package) (api.Package, error)
}

// Apply returns a filtered package, retaining existing provenance and recording
// selectors from a successful ForDeploy application, including within Combine.
func Apply(definition api.Package, filter ComponentFilterStrategy) (api.Package, error) {
	if filter == nil {
		filter = Empty()
	}

	if packageFilter, ok := filter.(packageFilterStrategy); ok {
		return packageFilter.applyPackage(definition)
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
	result, err := f.applyPackage(pkg)
	if err != nil {
		return nil, err
	}
	return result.Components, nil
}

func (f *comboFilter) applyPackage(pkg api.Package) (api.Package, error) {
	result := pkg
	for _, filter := range f.filters {
		var err error
		result, err = Apply(result, filter)
		if err != nil {
			return api.Package{}, fmt.Errorf("error applying filter %T: %w", filter, err)
		}
	}
	return result, nil
}

// Combine creates a new filter that applies a sequence of filters.
func Combine(filters ...ComponentFilterStrategy) ComponentFilterStrategy {
	return &comboFilter{filters}
}
