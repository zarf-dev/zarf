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

// NormalizeComponentSelectors normalizes selectors while preserving their order and repetitions.
func NormalizeComponentSelectors(selectors string) (string, error) {
	parsed, err := parseComponentSelectors(selectors)
	if err != nil {
		return "", err
	}
	return strings.Join(parsed, ","), nil
}

func parseComponentSelectors(selectors string) ([]string, error) {
	parts := strings.Split(selectors, ",")
	result := parts[:0]
	for _, selector := range parts {
		selector = strings.ToLower(strings.TrimSpace(selector))
		if selector == "" {
			continue
		}
		pattern := strings.TrimPrefix(selector, "-")
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, err
		}
		result = append(result, selector)
	}
	return result, nil
}

// includedOrExcluded records every matching selector; the last match determines the state.
func includedOrExcluded(componentName string, requestedComponentNames []string, matchedRequests map[string]bool) (selectState, error) {
	state := unknown
	for _, requestedComponent := range requestedComponentNames {
		isExclusion := strings.HasPrefix(requestedComponent, "-")
		pattern := strings.TrimPrefix(requestedComponent, "-")
		// Use path.Match for consistent glob behavior across operating systems.
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
		if isExclusion {
			state = excluded
		} else {
			state = included
		}
	}
	return state, nil
}
