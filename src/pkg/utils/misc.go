// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"fmt"
	"regexp"
	"time"
)

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

// Retry will retry a function until it succeeds or the timeout is reached, timeout == retries * delay.
func Retry(fn func() error, retries int, delay time.Duration) (err error) {
	for r := 0; r < retries; r++ {
		err = fn()
		if err == nil {
			break
		}

		time.Sleep(delay)
	}

	return err
}

// SliceContains returns true if the given element is in the slice.
func SliceContains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

// MergeMap merges map m2 with m1 overwriting common values with m2's values.
func MergeMap[T any](m1 map[string]T, m2 map[string]T) (r map[string]T) {
	r = map[string]T{}

	for key, value := range m1 {
		r[key] = value
	}

	for key, value := range m2 {
		r[key] = value
	}

	return r
}

// TransformMapKeys takes a map and transforms its keys using the provided function.
func TransformMapKeys[T any](m map[string]T, transform func(string) string) (r map[string]T) {
	r = map[string]T{}

	for key, value := range m {
		r[transform(key)] = value
	}

	return r
}

// MatchRegex wraps a get function around a substring match.
func MatchRegex(regex *regexp.Regexp, str string) (func(string) string, error) {
	// Validate the string.
	matches := regex.FindStringSubmatch(str)

	// Parse the string into its components.
	get := func(name string) string {
		return matches[regex.SubexpIndex(name)]
	}

	if len(matches) == 0 {
		return get, fmt.Errorf("unable to match against %s", str)
	}

	return get, nil
}
