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

// NormalizeComponentSelectors normalizes a comma-separated component selector list while preserving its order.
func NormalizeComponentSelectors(selectors string) (string, error) {
	result := make([]string, 0)
	for _, selector := range strings.Split(selectors, ",") {
		selector = strings.ToLower(strings.TrimSpace(selector))
		if selector == "" {
			continue
		}
		pattern := strings.TrimPrefix(selector, "-")
		if _, err := path.Match(pattern, ""); err != nil {
			return "", err
		}
		result = append(result, selector)
	}
	return strings.Join(result, ","), nil
}

func includedOrExcluded(componentName string, requestedComponentNames []string, matchedRequests map[string]bool) (selectState, error) {
	result := unknown
	for _, requestedComponent := range requestedComponentNames {
		pattern := strings.TrimPrefix(requestedComponent, "-")
		matched, err := path.Match(pattern, componentName)
		if err != nil {
			return unknown, err
		}
		if !matched {
			continue
		}

		if matchedRequests != nil {
			matchedRequests[requestedComponent] = true
		}
		if strings.HasPrefix(requestedComponent, "-") {
			result = excluded
		} else {
			result = included
		}
	}
	return result, nil
}
