// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
)

func TestYOLOMode(t *testing.T) {
	t.Log("E2E: YOLO Mode")

	// Don't run this test in appliance mode
	if e2e.ApplianceMode {
		return
	}

	// Destroy the cluster to test Zarf cleaning up after itself
	stdOut, stdErr, err := e2e.Zarf(t, "destroy", "--confirm", "--remove-components")
	require.NoError(t, err, stdOut, stdErr)

	tmpdir := t.TempDir()
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", "examples/yolo", "-o", tmpdir)
	require.NoError(t, err, stdOut, stdErr)

	packageName := fmt.Sprintf("zarf-package-yolo-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// Deploy the YOLO package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	c, err := cluster.New(t.Context())
	require.NoError(t, err)
	tunnel, err := c.Connect(context.Background(), "doom")
	require.NoError(t, err)
	defer tunnel.Close()

	// Check that 'curl' returns something.
	resp, err := http.Get(tunnel.HTTPEndpoint())
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "yolo", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestDevDeploy(t *testing.T) {
	// Don't run this test in appliance mode
	if e2e.ApplianceMode {
		return
	}

	// Generic test of dev deploy
	stdOut, stdErr, err := e2e.Zarf(t, "dev", "deploy", "examples/dos-games")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "kubectl", "delete", "namespace", "dos-games")
	require.NoError(t, err, stdOut, stdErr)

	// Special test of hidden registry-url flag
	stdOut, stdErr, err = e2e.Zarf(t, "dev", "deploy", "src/test/packages/99-registry-url", "--registry-url", "ghcr.io")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "kubectl", "delete", "namespace", "registry-url")
	require.NoError(t, err, stdOut, stdErr)
}
