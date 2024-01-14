// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

var (
	_ ComponentFilterStrategy = &ArchAndOSFilter{}
)

// NewArchAndOSFilter creates an architecture and OS filter.
func NewArchAndOSFilter(arch string, os string) *ArchAndOSFilter {
	return &ArchAndOSFilter{
		arch: arch,
		os:   os,
	}
}

// ArchAndOSFilter filters components based on OS/architecture.
type ArchAndOSFilter struct {
	arch string
	os   string
}

// Apply applies the filter.
func (f *ArchAndOSFilter) Apply(pkg types.ZarfPackage) ([]types.ZarfComponent, error) {
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
