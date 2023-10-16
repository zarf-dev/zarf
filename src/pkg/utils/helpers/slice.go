// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

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
