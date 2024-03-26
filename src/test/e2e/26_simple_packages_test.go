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

	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/stretchr/testify/require"
)

func TestDosGames(t *testing.T) {
	t.Log("E2E: Dos games")
	e2e.SetupWithCluster(t)

	path := filepath.Join("build", fmt.Sprintf("zarf-package-dos-games-%s-1.0.0.tar.zst", e2e.Arch))

	// Deploy the game
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	c, err := cluster.NewCluster()
	require.NoError(t, err)
	tunnel, err := c.Connect(context.TODO(), "doom")
	require.NoError(t, err)
	defer tunnel.Close()

	// Check that 'curl' returns something.
	resp, err := http.Get(tunnel.HTTPEndpoint())
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, err = e2e.Zarf("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	testCreate := filepath.Join("src", "test", "packages", "26-image-dos-games")
	testDeploy := filepath.Join("build", fmt.Sprintf("zarf-package-dos-games-images-%s.tar.zst", e2e.Arch))

	// Create the game image test package
	stdOut, stdErr, err = e2e.Zarf("package", "create", testCreate, "-o", "build", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy the game image test package
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", testDeploy, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestManifests(t *testing.T) {
	t.Log("E2E: Local, Remote, and Kustomize Manifests")
	e2e.SetupWithCluster(t)

	path := filepath.Join("build", fmt.Sprintf("zarf-package-manifests-%s-0.0.1.tar.zst", e2e.Arch))

	// Deploy the package
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the package
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "manifests", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
