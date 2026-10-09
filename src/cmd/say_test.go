// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderLogo(t *testing.T) {
	t.Parallel()

	const esc = "\x1b"

	colored := renderLogo(true)
	require.Equal(t, logo(), colored)
	require.Contains(t, colored, esc)

	plain := renderLogo(false)
	require.NotContains(t, plain, esc, "color codes must be stripped when color is disabled")
	require.Less(t, len(plain), len(colored))
	// Stripping removes only the color codes, never the art itself.
	require.NotEmpty(t, strings.TrimSpace(plain))
	require.Equal(t, strings.Count(colored, "&"), strings.Count(plain, "&"))
	require.Equal(t, strings.Count(colored, "\n"), strings.Count(plain, "\n"))
}
