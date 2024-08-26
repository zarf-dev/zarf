// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"errors"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// ByLocalOS creates a new filter that filters components based on local (runtime) OS.
func ByLocalOS(localOS string) ComponentFilterStrategy {
	return &localOSFilter{localOS}
}

// localOSFilter filters components based on local (runtime) OS.
type localOSFilter struct {
	localOS string
}

// ErrLocalOSRequired is returned when localOS is not set.
var ErrLocalOSRequired = errors.New("localOS is required")

// Apply applies the filter.
func (f *localOSFilter) Apply(pkg v1alpha1.ZarfPackage) ([]v1alpha1.ZarfComponent, error) {
	if f.localOS == "" {
		return nil, ErrLocalOSRequired
	}

	filtered := []v1alpha1.ZarfComponent{}
	for _, component := range pkg.Components {
		if component.Only.LocalOS == "" || component.Only.LocalOS == f.localOS {
			filtered = append(filtered, component)
		}
	}
	return filtered, nil
}
