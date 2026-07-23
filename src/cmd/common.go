// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import "maps"

// mergeMap returns a new map containing base's entries with override's copied on top.
// base is not mutated.
func mergeMap[T any](base, override map[string]T) map[string]T {
	merged := maps.Clone(base)
	if merged == nil {
		merged = make(map[string]T)
	}
	maps.Copy(merged, override)
	return merged
}
