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

func TestLegacySBOMArchiveFiles(t *testing.T) {
	t.Parallel()

	files, err := legacySBOMArchiveFiles([]string{
		"component:metrics",
		"image:docker.io/library/nginx:1.27-linux-amd64",
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		"zarf-component-metrics.json",
		"docker.io_library_nginx_1.27-linux-amd64.json",
	}, files)

	_, err = legacySBOMArchiveFiles([]string{"component:metrics", "image:zarf-component-metrics"})
	require.ErrorContains(t, err, `both normalize to "zarf-component-metrics.json"`)

	_, err = legacySBOMArchiveFiles([]string{"image:"})
	require.ErrorContains(t, err, `invalid legacy SBOM image key "image:"`)
}
