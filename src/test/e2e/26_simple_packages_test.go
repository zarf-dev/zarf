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
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/test"
)

func TestDosGames(t *testing.T) {
	t.Log("E2E: Dos games")

	tmpdir := t.TempDir()

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "examples/dos-games", "-o", tmpdir, "--skip-sbom")
	require.NoError(t, err, stdOut, stdErr)
	packageName := fmt.Sprintf("zarf-package-dos-games-%s-1.1.0.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// Deploy the game
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	c, err := cluster.NewCluster()
	require.NoError(t, err)
	ctx := logger.WithContext(context.Background(), test.GetLogger(t))
	tunnel, err := c.Connect(ctx, "doom")
	require.NoError(t, err)
	defer tunnel.Close()

	// Check that 'curl' returns something.
	resp, err := http.Get(tunnel.HTTPEndpoint())
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	testCreate := filepath.Join("src", "test", "packages", "26-image-dos-games")
	gamesPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-dos-games-images-%s.tar.zst", e2e.Arch))

	// Create the game image test package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", testCreate, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the game image test package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", gamesPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	_, _, err = e2e.Zarf(t, "package", "remove", gamesPath, "--confirm")
	require.NoError(t, err)
}

func TestManifests(t *testing.T) {
	t.Log("E2E: Local, Remote, and Kustomize Manifests")

	tmpdir := t.TempDir()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "examples/manifests", "-o", tmpdir, "--skip-sbom", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	path := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-manifests-%s-0.0.1.tar.zst", e2e.Arch))

	// Deploy the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "manifests", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestAgentIgnore(t *testing.T) {
	t.Log("E2E: Test Manifests that are Agent Ignored")

	tmpdir := t.TempDir()
	testCreate := filepath.Join("src", "test", "packages", "26-agent-ignore")
	testDeploy := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-agent-ignore-namespace-%s.tar.zst", e2e.Arch))

	// Create the agent ignore test package
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testCreate, "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the agent ignore test package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", testDeploy, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	_, _, err = e2e.Zarf(t, "package", "remove", testDeploy, "--confirm")
	require.NoError(t, err)
}
