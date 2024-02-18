// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"fmt"
	"slices"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// ByArchAndOS creates an architecture and OS filter.
func ByArchAndOS(arch string, os string) ComponentFilterStrategy {
	return &archAndOSFilter{arch, os}
}

// archAndOSFilter filters components based on OS/architecture.
type archAndOSFilter struct {
	arch string
	os   string
}

// Define allowed OS and architecture, an empty string means it is allowed on all platforms.
// same as enums on ZarfComponentOnlyTarget
var allowedOs = []string{"linux", "darwin", "windows", ""}

// same as enums on ZarfComponentOnlyClusterTarget
var allowedArch = []string{"amd64", "arm64", ""}

// Apply applies the filter.
func (f *archAndOSFilter) Apply(pkg types.ZarfPackage) ([]types.ZarfComponent, error) {
	if !slices.Contains(allowedOs, f.os) {
		return nil, fmt.Errorf("invalid OS: %s", f.os)
	}

	if !slices.Contains(allowedArch, f.arch) {
		return nil, fmt.Errorf("invalid architecture: %s", f.arch)
	}

	filtered := []types.ZarfComponent{}
	// Filter each component to only compatible platforms.
	for _, component := range pkg.Components {
		// Ignore only filters that are empty
		var validArch, validOS bool

		// Test for valid architecture
		if component.Only.Cluster.Architecture == "" || component.Only.Cluster.Architecture == f.arch {
			validArch = true
		} else {
			message.Debugf("Skipping component %s, %s is not compatible with %s", component.Name, component.Only.Cluster.Architecture, f.arch)
		}

		// Test for a valid OS
		if component.Only.LocalOS == "" || component.Only.LocalOS == f.os {
			validOS = true
		} else {
			message.Debugf("Skipping component %s, %s is not compatible with %s", component.Name, component.Only.LocalOS, f.os)
		}

		// If both the OS and architecture are valid, add the component to the filtered list
		if validArch && validOS {
			filtered = append(filtered, component)
		}
	}
	return filtered, nil
}
