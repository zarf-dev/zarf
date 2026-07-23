// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package helpers

import "strings"

// Unique returns a new slice with only unique elements.
func Unique[T comparable](s []T) (r []T) {
	exists := make(map[T]bool)
	for _, str := range s {
		if _, ok := exists[str]; !ok {
			exists[str] = true
			r = append(r, str)
		}
	}
	return r
}

// Reverse returns a new slice with the elements in reverse order.
func Reverse[T any](s []T) (r []T) {
	for i := len(s) - 1; i >= 0; i-- {
		r = append(r, s[i])
	}
	return r
}

// Filter returns a new slice with only the elements that pass the test.
func Filter[T any](ss []T, test func(T) bool) (r []T) {
	if test == nil {
		return ss
	}
	for _, s := range ss {
		if test(s) {
			r = append(r, s)
		}
	}
	return r
}

// Find returns the first element that passes the test.
func Find[T any](ss []T, test func(T) bool) (r T) {
	for _, s := range ss {
		if test(s) {
			return s
		}
	}
	return r
}

// RemoveMatches removes the given element from the slice that matches the test.
func RemoveMatches[T any](ss []T, test func(T) bool) (r []T) {
	for _, s := range ss {
		if !test(s) {
			r = append(r, s)
		}
	}
	return r
}

// StringToSlice converts a comma-separated string to a slice of lowercase strings.
func StringToSlice(s string) []string {
	if s != "" {
		split := strings.Split(s, ",")
		for idx, element := range split {
			split[idx] = strings.ToLower(strings.TrimSpace(element))
		}
		return split
	}

	return []string{}
}

// EqualFunc defines a type for a function that determines equality between two elements of type T.
type EqualFunc[T any] func(a, b T) bool

// MergeSlices merges two slices, s1 and s2, and returns a new slice containing all elements from s1
// and only those elements from s2 that do not exist in s1 based on the provided equal function.
func MergeSlices[T any](s1, s2 []T, equals EqualFunc[T]) []T {
	merged := make([]T, 0, len(s1)+len(s2))
	merged = append(merged, s1...)

	for _, v2 := range s2 {
		exists := false
		for _, v1 := range s1 {
			if equals(v1, v2) {
				exists = true
				break
			}
		}
		if !exists {
			merged = append(merged, v2)
		}
	}

	return merged
}
