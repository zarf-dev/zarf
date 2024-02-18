// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// ByIncluded creates a new simple included filter.
func ByIncluded(optionalComponents string) ComponentFilterStrategy {
	requested := helpers.StringToSlice(optionalComponents)

	return &includedFilter{
		requested,
	}
}

// includedFilter sorts based purely on the internal included state of the component.
type includedFilter struct {
	requestedComponents []string
}

// Apply applies the filter.
func (f *includedFilter) Apply(pkg types.ZarfPackage) ([]types.ZarfComponent, error) {
	isPartial := len(f.requestedComponents) > 0 && f.requestedComponents[0] != ""

	result := []types.ZarfComponent{}

	for _, component := range pkg.Components {
		selectState := unknown

		if isPartial {
			selectState, _ = includedOrExcluded(component.Name, f.requestedComponents)

			if selectState == excluded {
				continue
			}
		} else {
			selectState = included
		}

		if selectState == included {
			result = append(result, component)
		}
	}

	return result, nil
}
