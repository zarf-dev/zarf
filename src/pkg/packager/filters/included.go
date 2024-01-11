// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	_ ComponentFilterStrategy = &IncludedFilter{}
)

// NewIncludedFilter creates a new simple included filter.
func NewIncludedFilter(optionalComponents string) *IncludedFilter {
	requested := helpers.StringToSlice(optionalComponents)

	return &IncludedFilter{
		requested,
	}
}

// IncludedFilter sorts based purely on the internal included state of the component.
type IncludedFilter struct {
	requestedComponents []string
}

// Apply applies the filter.
func (f *IncludedFilter) Apply(allComponents []types.ZarfComponent) ([]types.ZarfComponent, error) {
	isPartial := len(f.requestedComponents) > 0 && f.requestedComponents[0] != ""

	result := []types.ZarfComponent{}

	for _, component := range allComponents {
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
