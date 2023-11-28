// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBadRemotePackages(t *testing.T) {
	t.Log("E2E: test bad remote packages")

	zarfYaml := `
kind: ZarfPackageConfig
metadata:
  name: doesnotexist
components:
  - name: doesnotexist-docker
    required: true
    images:
      - localhost/doesnotexist:6.13371337
`
	t.Run("zarf package create bad images", func(t *testing.T) {
		// Create a temporary directory
		tmpDir, err := os.MkdirTemp("", "test")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir) // Clean up

		// Create zarf.yaml in the temporary directory
		yamlPath := filepath.Join(tmpDir, "zarf.yaml")
		require.NoError(t, os.WriteFile(yamlPath, []byte(zarfYaml), 0600))

		// Run the e2e.Zarf command
		_, stdErr, err := e2e.Zarf("package", "create", tmpDir, "--confirm")
		require.Error(t, err)

		// Check the standard error output
		require.Contains(t, stdErr, "Name:localhost/doesnotexist")
	})
}
