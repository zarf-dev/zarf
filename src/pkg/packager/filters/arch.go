// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package filters

import (
	"errors"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// ByArchitecture creates a filter that drops components whose only.cluster.architecture
// is set and does not match the given host architecture.
func ByArchitecture(hostArch string) ComponentFilterStrategy {
	return &clusterArchFilter{hostArch}
}

type clusterArchFilter struct {
	hostArch string
}

// ErrHostArchRequired is returned when hostArch is not set.
var ErrHostArchRequired = errors.New("hostArch is required")

// Apply applies the filter.
func (f *clusterArchFilter) Apply(pkg v1alpha1.ZarfPackage) ([]v1alpha1.ZarfComponent, error) {
	if f.hostArch == "" {
		return nil, ErrHostArchRequired
	}
	filtered := []v1alpha1.ZarfComponent{}
	for _, component := range pkg.Components {
		if component.Only.Cluster.Architecture == "" || component.Only.Cluster.Architecture == f.hostArch {
			filtered = append(filtered, component)
		}
	}
	return filtered, nil
}
