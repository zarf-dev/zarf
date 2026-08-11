// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	internaltypes "github.com/zarf-dev/zarf/src/internal/api/types"
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

// Apply applies a component filter to a package definition.
func Apply(definition api.PackageDefinition, filter ComponentFilterStrategy) (api.PackageDefinition, error) {
	if filter == nil {
		filter = Empty()
	}

	indices, err := filter.Apply(packageView(definition))
	if err != nil {
		return api.PackageDefinition{}, err
	}

	components := make([]internaltypes.Component, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(definition.Pkg.Components) {
			return api.PackageDefinition{}, fmt.Errorf("filter returned index %d out of range", idx)
		}
		components = append(components, definition.Pkg.Components[idx])
	}
	definition.Pkg.Components = components
	return definition, nil
}

func packageView(definition api.PackageDefinition) PackageView {
	definitions := componentDefinitionsForDisplay(definition)
	components := make([]ComponentView, 0, len(definition.Pkg.Components))
	for idx, component := range definition.Pkg.Components {
		components = append(components, ComponentView{
			Name:        component.Name,
			Description: component.Description,
			Optional:    component.Optional,
			Default:     component.Default,
			Group:       component.Group,
			OnlyLocalOS: component.Target.OS,
			Definition:  definitions[idx],
		})
	}
	return PackageView{Components: components}
}

func componentDefinitionsForDisplay(definition api.PackageDefinition) []any {
	definitions := make([]any, len(definition.Pkg.Components))
	if definition.OriginalAPIVersion() == v1beta1.APIVersion {
		for idx, component := range definition.AsV1beta1().Components {
			definitions[idx] = component
		}
		return definitions
	}
	for idx, component := range definition.AsV1alpha1().Components {
		definitions[idx] = component
	}
	return definitions
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
