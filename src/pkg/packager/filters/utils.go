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
	// In unmatched cases we don't know if we should include or exclude yet
	var match string
	var matchType = unknown

	// Check every component request, so last match get's priority
	// Order is assumed to match user input
	for _, requestedComponent := range requestedComponentNames {
		// If the component glob matches one of the requested components, then return true
		// This supports globbing with "path" in order to have the same behavior across OSes (if we ever allow namespaced components with /)
		matched, err := path.Match(strings.TrimPrefix(requestedComponent, "-"), componentName)
		if err != nil {
			return unknown, "", err
		}

		if matched {
			match = requestedComponent
			// Exclusions are requests with the leading "-"
			if strings.HasPrefix(requestedComponent, "-") {
				matchType = excluded
			} else {
				matchType = included
			}
		}
	}

	return matchType, match, nil
}
