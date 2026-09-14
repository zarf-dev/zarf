// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"github.com/defenseunicorns/pkg/helpers/v2"
)

// BySelectState creates a new simple included filter.
func BySelectState(optionalComponents string) ComponentFilterStrategy {
	requested := helpers.StringToSlice(optionalComponents)

	return &selectStateFilter{
		requested,
	}
}

// selectStateFilter sorts based purely on the internal included state of the component.
type selectStateFilter struct {
	requestedComponents []string
}

// Apply applies the filter.
func (f *selectStateFilter) Apply(pkg PackageView) ([]int, error) {
	isPartial := len(f.requestedComponents) > 0 && f.requestedComponents[0] != ""
	result := []int{}
	for idx, component := range pkg.Components {
		selectState := included
		if isPartial {
			var err error
			selectState, _, err = includedOrExcluded(component.Name, f.requestedComponents)
			if err != nil {
				return nil, err
			}
		}
		if selectState != included {
			continue
		}
		result = append(result, idx)
	}
	return result, nil
}
