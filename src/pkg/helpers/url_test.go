// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package helpers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURLHelpers(t *testing.T) {
	require.True(t, IsURL("https://example.com/path"))
	require.False(t, IsURL("example.com/path"))
	require.True(t, IsOCIURL("oci://registry.example.com/package:1"))

	base, err := ExtractBasePathFromURL("https://example.com/path/package.tar.zst")
	require.NoError(t, err)
	require.Equal(t, "package.tar.zst", base)
}
