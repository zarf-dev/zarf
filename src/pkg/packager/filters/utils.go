// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package filters contains core implementations of the ComponentFilterStrategy interface.
package filters

import (
	"path"
	"slices"
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
	parsed, err := ParseComponentSelectors(selectors)
	if err != nil {
		return "", err
	}
	return strings.Join(parsed, ","), nil
}

// ParseComponentSelectors parses comma-separated selectors into a normalized,
// ordered list, preserving repetitions.
func ParseComponentSelectors(selectors string) ([]string, error) {
	if selectors == "" {
		return nil, nil
	}
	return normalizeComponentSelectorList(strings.Split(selectors, ","))
}

// NormalizeComponentSelectorList returns a detached, normalized selector list.
// Entries are individual patterns, not comma-separated strings. Order and
// repetitions are preserved, and empty entries are discarded.
func NormalizeComponentSelectorList(selectors []string) ([]string, error) {
	return normalizeComponentSelectorList(slices.Clone(selectors))
}

// normalizeComponentSelectorList compacts and normalizes an owned slice in place.
func normalizeComponentSelectorList(selectors []string) ([]string, error) {
	result := selectors[:0]
	for _, selector := range selectors {
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
	if len(result) == 0 {
		return nil, nil
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
