// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import "strings"

// isJSONPathWaitType checks if the condition is a JSONPath or condition.
func IsJSONPathWaitType(condition string) bool {
	if condition[0] != '{' || !strings.Contains(condition, "=") || !strings.Contains(condition, "}") {
		return false
	}

	return true
}
