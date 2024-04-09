// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateIndexSha(t *testing.T) {
	t.Log("E2E: Create Templating")

	_, stderr, err := e2e.Zarf("package", "create", "src/test/packages/14-index-sha", "--confirm")
	// Not sure why this isn't working
	require.Error(t, err)
	require.Contains(t, stderr, "docker.io/defenseunicorns/zarf-game:multi-tile-dark@sha256:f78e442f0f3eb3e9459b5ae6b1a8fda62f8dfe818112e7d130a4e8ae72b3cbff")

}
