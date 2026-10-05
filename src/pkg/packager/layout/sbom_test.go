// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSBOMResourcePathRoundTrip(t *testing.T) {
	t.Parallel()

	require.Equal(t, "sboms/files/metrics.json", SBOMResourcePath("component:metrics"))

	for _, key := range []string{
		"component:metrics",
		"image:registry.example/app:1.2.3/linux/amd64",
	} {
		t.Run(key, func(t *testing.T) {
			path := SBOMResourcePath(key)
			actual, ok := SBOMResourceKey(path)
			require.True(t, ok)
			require.Equal(t, key, actual)
		})
	}
}
