// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import (
	"fmt"
	"reflect"
	"regexp"
	"time"
)

// Retry will retry a function until it succeeds or the timeout is reached, timeout == retries * delay.
func Retry(fn func() error, retries int, delay time.Duration, logger func(format string, args ...any)) (err error) {
	for r := 0; r < retries; r++ {
		err = fn()
		if err == nil {
			break
		}

		logger("Retrying (%d/%d): %s", r+1, retries, err.Error())

		time.Sleep(delay)
	}

	return err
}

// MergeMap merges map m2 with m1 overwriting common values with m2's values.
func MergeMap[T any](m1, m2 map[string]T) (r map[string]T) {
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

// TransformAndMergeMap transforms keys in both maps then merges map m2 with m1 overwriting common values with m2's values.
func TransformAndMergeMap[T any](m1, m2 map[string]T, transform func(string) string) (r map[string]T) {
	mt1 := TransformMapKeys(m1, transform)
	mt2 := TransformMapKeys(m2, transform)
	r = MergeMap(mt1, mt2)

	return r
}

// MergeMapRecursive recursively (nestedly) merges map m2 with m1 overwriting common values with m2's values.
func MergeMapRecursive(m1, m2 map[string]interface{}) (r map[string]interface{}) {
	r = map[string]interface{}{}

	for key, value := range m1 {
		r[key] = value
	}

	for key, value := range m2 {
		if value, ok := value.(map[string]interface{}); ok {
			if nestedValue, ok := r[key]; ok {
				if nestedValue, ok := nestedValue.(map[string]interface{}); ok {
					r[key] = MergeMapRecursive(nestedValue, value)
					continue
				}
			}
		}
		r[key] = value
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

// IsNotZeroAndNotEqual is used to test if a struct has zero values or is equal values with another struct
func IsNotZeroAndNotEqual[T any](given T, equal T) bool {
	givenValue := reflect.ValueOf(given)
	equalValue := reflect.ValueOf(equal)

	if givenValue.NumField() != equalValue.NumField() {
		return true
	}

	for i := 0; i < givenValue.NumField(); i++ {
		if !givenValue.Field(i).IsZero() &&
			givenValue.Field(i).CanInterface() &&
			givenValue.Field(i).Interface() != equalValue.Field(i).Interface() {

			return true
		}
	}
	return false
}

// MergeNonZero is used to merge non-zero overrides from one struct into another of the same type
func MergeNonZero[T any](original T, overrides T) T {
	originalValue := reflect.ValueOf(&original)
	overridesValue := reflect.ValueOf(&overrides)

	for i := 0; i < originalValue.Elem().NumField(); i++ {
		if !overridesValue.Elem().Field(i).IsZero() &&
			overridesValue.Elem().Field(i).CanSet() {

			overrideField := overridesValue.Elem().Field(i)
			originalValue.Elem().Field(i).Set(overrideField)
		}
	}
	return originalValue.Elem().Interface().(T)
}
