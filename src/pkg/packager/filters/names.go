// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters

import "github.com/zarf-dev/zarf/src/api"

// ByComponentNames selects components with names in names.
func ByComponentNames(names []string) ComponentFilterStrategy {
	selected := make(map[string]struct{}, len(names))
	for _, name := range names {
		selected[name] = struct{}{}
	}
	return componentNamesFilter{selected: selected}
}

type componentNamesFilter struct {
	selected map[string]struct{}
}

// Apply returns selected components in package order.
func (f componentNamesFilter) Apply(pkg api.Package) ([]api.Component, error) {
	components := make([]api.Component, 0, len(f.selected))
	for _, component := range pkg.Components {
		if _, found := f.selected[component.Name]; found {
			components = append(components, component)
		}
	}
	return components, nil
}
