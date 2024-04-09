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

	_, stderr, err := e2e.Zarf("package", "create", "src/test/packages/14-index-sha/image-index", "--confirm")
	require.Error(t, err)
	require.Contains(t, stderr, "ghcr.io/defenseunicorns/zarf/agent:v0.32.6@sha256:b3fabdc7d4ecd0f396016ef78da19002c39e3ace352ea0ae4baa2ce9d5958376")

	_, stderr, err = e2e.Zarf("package", "create", "src/test/packages/14-index-sha/manifest-list", "--confirm")
	require.Error(t, err)
	require.Contains(t, stderr, "docker.io/defenseunicorns/zarf-game@sha256:f78e442f0f3eb3e9459b5ae6b1a8fda62f8dfe818112e7d130a4e8ae72b3cbff")

}
