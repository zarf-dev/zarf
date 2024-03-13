// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"runtime"

	"github.com/defenseunicorns/zarf/src/types"
)

func ByLocalOS() ComponentFilterStrategy {
	return &localOSFilter{}
}

// localOSFilter filters components based on local (runtime) OS.
type localOSFilter struct{}

// Apply applies the filter.
func (f *localOSFilter) Apply(pkg types.ZarfPackage) ([]types.ZarfComponent, error) {
	localOS := runtime.GOOS

	filtered := []types.ZarfComponent{}
	for _, component := range pkg.Components {
		if component.Only.LocalOS == "" || component.Only.LocalOS == localOS {
			filtered = append(filtered, component)
		}
	}
	return filtered, nil
}
