// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package helpers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// BoolPtr returns a pointer to a bool.
func BoolPtr(b bool) *bool {
	return &b
}

// RetryWithContext retries a function until it succeeds, the timeout is reached, or the context is done.
// The delay between attempts increases exponentially as (2^(attempt-1)) * delay.
// For example, with a delay of one second and three attempts, the timing would be:
// - First attempt: immediate
// - Second attempt: after one second
// - Third attempt: after two seconds
func RetryWithContext(ctx context.Context, fn func() error, attempts int, delay time.Duration, logger func(format string, args ...any)) error {
	if attempts < 1 {
		return errors.New("invalid number of attempts, must be at least 1")
	}
	var err error
	timer := time.NewTimer(0)
	defer timer.Stop()
	for r := range attempts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			err = fn()
			if err == nil {
				return nil
			}

			logger("Attempt (%d/%d) failed with: %s", r+1, attempts, err.Error())

			// No reason to wait when we aren't going to retry again
			if r+1 == attempts {
				return err
			}

			pow := math.Pow(2, float64(r))
			backoff := delay * time.Duration(pow)

			logger("Retrying in %s", backoff)

			timer.Reset(backoff)
		}
	}

	return err
}

// Retry retries a function until it succeeds, the timeout is reached, or the context is done.
// The delay between attempts increases exponentially as (2^(attempt-1)) * delay.
// For example, with a delay of one second and three attempts, the timing would be:
// - First attempt: immediate
// - Second attempt: after one second
// - Third attempt: after two seconds
func Retry(fn func() error, attempts int, delay time.Duration, logger func(format string, args ...any)) error {
	return RetryWithContext(context.Background(), fn, attempts, delay, logger)
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
	r = TransformMapKeys(m1, transform)
	mt2 := TransformMapKeys(m2, transform)
	maps.Copy(r, mt2)
	return r
}

// MergeMapRecursive recursively (nestedly) merges map m2 with m1 overwriting common values with m2's values.
func MergeMapRecursive(m1, m2 map[string]any) (r map[string]any) {
	r = maps.Clone(m1)

	if r == nil {
		r = map[string]any{}
	}

	for key, value := range m2 {
		if value, ok := value.(map[string]any); ok {
			if nestedValue, ok := r[key]; ok {
				if nestedValue, ok := nestedValue.(map[string]any); ok {
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

	for i := range givenValue.NumField() {
		if !givenValue.Field(i).IsZero() &&
			givenValue.Field(i).CanInterface() &&
			givenValue.Field(i).Interface() != equalValue.Field(i).Interface() {
			return true
		}
	}
	return false
}

// MergeNonZero is used to merge non-zero overrides from one struct into another of the same type
func MergeNonZero[T any](original, overrides T) T {
	// Create a copy of original that we'll modify
	result := original

	// Get reflect values, using the actual values not pointers to them
	resultValue := reflect.ValueOf(&result).Elem()
	overridesValue := reflect.ValueOf(overrides)

	// Ensure we're working with structs
	if resultValue.Kind() != reflect.Struct || overridesValue.Kind() != reflect.Struct {
		return original // Can't merge non-structs
	}

	// Iterate through fields
	for i := range resultValue.NumField() {
		resultField := resultValue.Field(i)
		overrideField := overridesValue.Field(i)

		// Check if override field is non-zero and result field can be set
		if !overrideField.IsZero() && resultField.CanSet() {
			resultField.Set(overrideField)
		}
	}

	return result
}

// MergePathAndValueIntoMap takes a path in dot notation as a string and a value (also as a string for simplicity),
// then merges this into the provided map. The value can be any type.
func MergePathAndValueIntoMap(m map[string]any, path string, value any) error {
	pathParts := strings.Split(path, ".")
	current := m
	for i, part := range pathParts {
		if i == len(pathParts)-1 {
			// Set the value at the last key in the path.
			current[part] = value
		} else {
			if _, exists := current[part]; !exists {
				// If the part does not exist, create a new map for it.
				current[part] = make(map[string]any)
			}

			nextMap, ok := current[part].(map[string]any)
			if !ok {
				return fmt.Errorf("conflict at %q, expected map but got %T", strings.Join(pathParts[:i+1], "."), current[part])
			}
			current = nextMap
		}
	}
	return nil
}
