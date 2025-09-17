// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/test"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestDosGames(t *testing.T) {
	t.Log("E2E: Dos games")
	ctx := logger.WithContext(t.Context(), test.GetLogger(t))

	tmpdir := t.TempDir()

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "examples/dos-games", "-o", tmpdir, "--skip-sbom")
	require.NoError(t, err, stdOut, stdErr)
	packageName := fmt.Sprintf("zarf-package-dos-games-%s-1.2.0.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// Deploy the game
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	c, err := cluster.New(ctx)
	require.NoError(t, err)
	tunnel, err := c.Connect(ctx, "doom")
	require.NoError(t, err)
	defer tunnel.Close()

	endpoints := tunnel.HTTPEndpoints()
	require.Len(t, endpoints, 1)

	// Check that 'curl' returns something.
	resp, err := http.Get(endpoints[0])
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

	// Deploy the package. Need to adopt existing resources to make sure that the label update for the package name
	// applies to any previously created namespaces
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--adopt-existing-resources")
	require.NoError(t, err, stdOut, stdErr)

	// Validate the namespaces are labeled
	stdOut, stdErr, err = e2e.Kubectl(t, "get", "namespaces", "-l", "zarf.dev/package=manifests", "-o", "json")
	require.NoError(t, err, stdOut, stdErr)

	namespaceList := &corev1.NamespaceList{}
	err = json.Unmarshal([]byte(stdOut), namespaceList)
	require.NoError(t, err)
	require.Len(t, namespaceList.Items, 3, "expected 3 namespaces")
	// validate the deployments
	stdOut, stdErr, err = e2e.Kubectl(t, "get", "deployments", "-l", "zarf.dev/package=manifests", "--all-namespaces", "-o", "json")
	require.NoError(t, err, stdOut, stdErr)

	deploymentList := &appsv1.DeploymentList{}
	err = json.Unmarshal([]byte(stdOut), deploymentList)
	require.NoError(t, err)
	require.Len(t, deploymentList.Items, 3, "expected 3 deployments")

	// Wait for all the deployments to be ready
	stdOut, stdErr, err = e2e.Kubectl(t, "wait", "deployment", "-l", "zarf.dev/package=manifests", "--for=condition=Available", "--all-namespaces", "--timeout=1m")
	require.NoError(t, err, stdOut, stdErr)

	// List pods by the zarf.dev/package label
	stdOut, stdErr, err = e2e.Kubectl(t, "get", "pods", "-l", "zarf.dev/package=manifests", "--all-namespaces", "-o", "json")
	require.NoError(t, err, stdOut, stdErr)

	podList := &corev1.PodList{}
	err = json.Unmarshal([]byte(stdOut), podList)
	require.NoError(t, err)

	require.Len(t, podList.Items, 6, "expected 6 pods")

	// Each deployment should have 2 replicas.
	podInfoCount := 0
	httpdCount := 0
	nginxCount := 0
	for _, pod := range podList.Items {
		if pod.Labels["app"] == "httpd" {
			httpdCount++
		}
		if pod.Labels["app"] == "nginx" {
			nginxCount++
		}
		if pod.Labels["app"] == "podinfo" {
			podInfoCount++
		}
	}
	require.Equal(t, 2, httpdCount)
	require.Equal(t, 2, nginxCount)
	require.Equal(t, 2, podInfoCount)
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
