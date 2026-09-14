// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

// Empty returns a filter that does nothing.
func Empty() ComponentFilterStrategy {
	return &emptyFilter{}
}

// emptyFilter is a filter that does nothing.
type emptyFilter struct{}

// Apply returns the components unchanged.
func (f *emptyFilter) Apply(pkg PackageView) ([]int, error) {
	indices := make([]int, len(pkg.Components))
	for idx := range pkg.Components {
		indices[idx] = idx
	}
	return indices, nil
}
