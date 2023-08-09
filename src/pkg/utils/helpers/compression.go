// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import (
	"strings"
)

func SupportedCompressionFormat(filename string) bool {
	supportedFormats := []string{".tar.gz", ".br", ".bz2", ".zip", ".lz4", ".sz", ".xz", ".zz", ".zst"}

	for _, format := range supportedFormats {
		if strings.HasSuffix(filename, format) {
			return true
		}
	}
	return false
}
