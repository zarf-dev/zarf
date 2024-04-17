// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateIndexShaErrors(t *testing.T) {
	t.Log("E2E: CreateIndexShaErrors")

	testCases := []struct {
		name                  string
		packagePath           string
		expectedImageInStderr string
	}{
		{
			name:                  "Image Index",
			packagePath:           "src/test/packages/14-index-sha/image-index",
			expectedImageInStderr: "ghcr.io/defenseunicorns/zarf/agent@sha256:b3fabdc7d4ecd0f396016ef78da19002c39e3ace352ea0ae4baa2ce9d5958376",
		},
		{
			name:                  "Manifest List",
			packagePath:           "src/test/packages/14-index-sha/manifest-list",
			expectedImageInStderr: "docker.io/defenseunicorns/zarf-game@sha256:f78e442f0f3eb3e9459b5ae6b1a8fda62f8dfe818112e7d130a4e8ae72b3cbff",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, err := e2e.Zarf("package", "create", tc.packagePath, "--confirm")
			require.Error(t, err)
			require.Contains(t, stderr, tc.expectedImageInStderr)
		})
	}

}
