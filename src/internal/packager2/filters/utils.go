// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"path"
	"strings"
)

type selectState int

const (
	unknown selectState = iota
	included
	excluded
)

func includedOrExcluded(componentName string, requestedComponentNames []string) (selectState, string, error) {
	// Check if the component has a leading dash indicating it should be excluded - this is done first so that exclusions precede inclusions
	for _, requestedComponent := range requestedComponentNames {
		if strings.HasPrefix(requestedComponent, "-") {
			// If the component glob matches one of the requested components, then return true
			// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
			matched, err := path.Match(strings.TrimPrefix(requestedComponent, "-"), componentName)
			if err != nil {
				return unknown, "", err
			}
			if matched {
				return excluded, requestedComponent, nil
			}
		}
	}
	// Check if the component matches a glob pattern and should be included
	for _, requestedComponent := range requestedComponentNames {
		// If the component glob matches one of the requested components, then return true
		// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
		matched, err := path.Match(requestedComponent, componentName)
		if err != nil {
			return unknown, "", err
		}
		if matched {
			return included, requestedComponent, nil
		}
	}

	// All other cases we don't know if we should include or exclude yet
	return unknown, "", nil
}
