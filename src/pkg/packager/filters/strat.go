// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
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

	if err := definition.RetainComponents(indices); err != nil {
		return api.PackageDefinition{}, fmt.Errorf("filter returned invalid component index: %w", err)
	}
	return definition, nil
}

func packageView(definition api.PackageDefinition) PackageView {
	v1alpha1Definition := definition.AsV1alpha1()
	v1beta1Definition := definition.AsV1beta1()
	components := make([]ComponentView, 0, len(v1alpha1Definition.Components))
	for idx, alphaComponent := range v1alpha1Definition.Components {
		betaComponent := v1beta1Definition.Components[idx]
		components = append(components, ComponentView{
			Name:        alphaComponent.Name,
			Description: alphaComponent.Description,
			Optional:    betaComponent.Optional,
			Default:     alphaComponent.Default,
			Group:       alphaComponent.DeprecatedGroup,
			OnlyLocalOS: betaComponent.Target.OS,
			Definition:  componentDefinitionForDisplay(definition, alphaComponent, betaComponent),
		})
	}
	return PackageView{Components: components}
}

func componentDefinitionForDisplay(definition api.PackageDefinition, v1alpha1Component v1alpha1.ZarfComponent, v1beta1Component v1beta1.Component) any {
	if definition.OriginalAPIVersion() == v1beta1.APIVersion {
		return v1beta1Component
	}
	return v1alpha1Component
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
