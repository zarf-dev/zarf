// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"github.com/zarf-dev/zarf/src/api"
)

// BySelectState creates a new simple included filter.
func BySelectState(optionalComponents string) ComponentFilterStrategy {
	requested, err := ParseComponentSelectors(optionalComponents)
	return &selectStateFilter{requestedComponents: requested, err: err}
}

// ByComponentSelectors selects components using an ordered list of names, globs,
// and exclusions. It takes a detached snapshot; invalid patterns fail on Apply.
func ByComponentSelectors(selectors []string) ComponentFilterStrategy {
	requested, err := NormalizeComponentSelectorList(selectors)
	return &selectStateFilter{requestedComponents: requested, err: err}
}

// selectStateFilter sorts based purely on the internal included state of the component.
type selectStateFilter struct {
	requestedComponents []string
	err                 error
}

// Apply applies the filter.
func (f *selectStateFilter) Apply(pkg api.Package) ([]api.Component, error) {
	if f.err != nil {
		return nil, f.err
	}
	isPartial := len(f.requestedComponents) > 0
	result := []api.Component{}
	for _, component := range pkg.Components {
		selectState := included
		if isPartial {
			var err error
			selectState, err = includedOrExcluded(component.Name, f.requestedComponents, nil)
			if err != nil {
				return nil, err
			}
		}
		if selectState != included {
			continue
		}
		result = append(result, component)
	}
	return result, nil
}
