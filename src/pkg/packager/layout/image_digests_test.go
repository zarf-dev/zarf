// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetImageDigests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		index       *string
		expected    map[string]string
		expectedErr string
	}{
		{
			name:     "no image index",
			expected: map[string]string{},
		},
		{
			name: "maps base image name annotations to digests",
			index: new(`{
				"schemaVersion": 2,
				"manifests": [
					{
						"mediaType": "application/vnd.oci.image.manifest.v1+json",
						"digest": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
						"size": 1,
						"annotations": {"org.opencontainers.image.base.name": "docker.io/library/nginx:1.27"}
					},
					{
						"mediaType": "application/vnd.oci.image.index.v1+json",
						"digest": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
						"size": 1,
						"annotations": {"org.opencontainers.image.base.name": "ghcr.io/zarf-dev/doom-game:0.0.1"}
					},
					{
						"mediaType": "application/vnd.oci.image.manifest.v1+json",
						"digest": "sha256:3333333333333333333333333333333333333333333333333333333333333333",
						"size": 1
					}
				]
			}`),
			expected: map[string]string{
				"docker.io/library/nginx:1.27":     "sha256:1111111111111111111111111111111111111111111111111111111111111111",
				"ghcr.io/zarf-dev/doom-game:0.0.1": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
			},
		},
		{
			name:        "malformed index",
			index:       new(`{not json`),
			expectedErr: "unable to parse image index",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tt.index != nil {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, ImagesDir), 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, IndexPath), []byte(*tt.index), 0o600))
			}
			pkgLayout := &PackageLayout{dirPath: dir}
			digests, err := pkgLayout.GetImageDigests()
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, digests)
		})
	}
}
