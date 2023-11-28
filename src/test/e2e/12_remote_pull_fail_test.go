// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBadRemotePackages(t *testing.T) {
	t.Log("E2E: test bad remote packages")

	t.Run("zarf package create bad images", func(t *testing.T) {
		_, stdErr, err := e2e.Zarf("package", "create", "src/test/packages/12-remote-pull-fail", "--confirm")
		// expecting zarf to have an error and output to stderr
		require.Error(t, err)
		require.Contains(t, stdErr, "requested access to the resource is denied")
	})
}
