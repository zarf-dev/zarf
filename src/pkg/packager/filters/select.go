// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"github.com/defenseunicorns/pkg/helpers"
	"github.com/defenseunicorns/zarf/src/types"
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
func (f *selectStateFilter) Apply(pkg types.ZarfPackage) ([]types.ZarfComponent, error) {
	isPartial := len(f.requestedComponents) > 0 && f.requestedComponents[0] != ""
	result := []types.ZarfComponent{}
	for _, component := range pkg.Components {
		selectState := included
		if isPartial {
			selectState, _ = includedOrExcluded(component.Name, f.requestedComponents)
		}
		if selectState != included {
			continue
		}
		result = append(result, component)
	}
	return result, nil
}
