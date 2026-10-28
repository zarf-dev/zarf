// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helpers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeMapRecursive(t *testing.T) {
	merged := MergeMapRecursive(
		map[string]any{"nested": map[string]any{"first": "value"}},
		map[string]any{"nested": map[string]any{"second": "override"}},
	)
	require.Equal(t, map[string]any{"nested": map[string]any{
		"first": "value", "second": "override",
	}}, merged)
}

func TestTruncate(t *testing.T) {
	result := Truncate(strings.Repeat("x", 10), 6, false)
	require.Equal(t, "xxx...", result)
}
