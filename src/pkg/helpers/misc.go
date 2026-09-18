// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helpers

import (
	"crypto/rand"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"strings"
)

// Very limited special chars for git / basic auth
// https://owasp.org/www-community/password-special-characters has complete list of safe chars.
const randomStringChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ!~-"

// RandomString generates a secure random string of the specified length.
func RandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for index, value := range bytes {
		bytes[index] = randomStringChars[value%byte(len(randomStringChars))]
	}
	return string(bytes), nil
}

// Unique returns a new slice with only unique elements.
func Unique[T comparable](values []T) (result []T) {
	seen := make(map[T]bool)
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

// StringToSlice converts a comma-separated string to a slice of lowercase strings.
func StringToSlice(value string) []string {
	if value == "" {
		return []string{}
	}
	result := strings.Split(value, ",")
	for index := range result {
		result[index] = strings.ToLower(strings.TrimSpace(result[index]))
	}
	return result
}

// TransformAndMergeMap transforms keys in both maps then merges map m2 with m1 overwriting common values with m2's values.
func TransformAndMergeMap[T any](first, second map[string]T, transform func(string) string) map[string]T {
	result := make(map[string]T, len(first)+len(second))
	for key, value := range first {
		result[transform(key)] = value
	}
	for key, value := range second {
		result[transform(key)] = value
	}
	return result
}

// MergeMapRecursive recursively (nestedly) merges map second with first overwriting common values with second's values.
func MergeMapRecursive(first, second map[string]any) map[string]any {
	result := maps.Clone(first)
	if result == nil {
		result = map[string]any{}
	}
	for key, value := range second {
		if nested, ok := value.(map[string]any); ok {
			if current, ok := result[key].(map[string]any); ok {
				result[key] = MergeMapRecursive(current, nested)
				continue
			}
		}
		result[key] = value
	}
	return result
}

// MatchRegex wraps a get function around a substring match.
func MatchRegex(regex *regexp.Regexp, value string) (func(string) string, error) {
	matches := regex.FindStringSubmatch(value)
	get := func(name string) string { return matches[regex.SubexpIndex(name)] }
	if len(matches) == 0 {
		return get, fmt.Errorf("unable to match against %s", value)
	}
	return get, nil
}

// IsNotZeroAndNotEqual is used to test if a struct has zero values or is equal values with another struct
func IsNotZeroAndNotEqual[T any](given, equal T) bool {
	givenValue, equalValue := reflect.ValueOf(given), reflect.ValueOf(equal)
	if givenValue.NumField() != equalValue.NumField() {
		return true
	}
	for index := range givenValue.NumField() {
		field := givenValue.Field(index)
		if !field.IsZero() && field.CanInterface() && field.Interface() != equalValue.Field(index).Interface() {
			return true
		}
	}
	return false
}

// MergeNonZero is used to merge non-zero overrides from one struct into another of the same type
func MergeNonZero[T any](original, overrides T) T {
	result := original
	resultValue := reflect.ValueOf(&result).Elem()
	overridesValue := reflect.ValueOf(overrides)
	if resultValue.Kind() != reflect.Struct || overridesValue.Kind() != reflect.Struct {
		return original
	}
	for index := range resultValue.NumField() {
		if field := overridesValue.Field(index); !field.IsZero() && resultValue.Field(index).CanSet() {
			resultValue.Field(index).Set(field)
		}
	}
	return result
}

// Truncate truncates provided text to the requested length
func Truncate(text string, length int, invert bool) string {
	escaped := strings.ReplaceAll(text, "\n", "; ")
	if len(escaped) <= length {
		return escaped
	}
	if invert {
		return "..." + escaped[len(escaped)-length+3:]
	}
	return escaped[:length-3] + "..."
}
