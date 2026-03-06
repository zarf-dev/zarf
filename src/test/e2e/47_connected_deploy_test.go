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
	"github.com/zarf-dev/zarf/src/pkg/state"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConnectedDeploy(t *testing.T) {
	t.Log("E2E: Connected Deploy")

	tmpdir := t.TempDir()

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", filepath.Join("src", "test", "packages", "47-connected-deploy"), "-o", tmpdir, "--confirm", "--skip-sbom")
	require.NoError(t, err, stdOut, stdErr)

	pkgPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-connected-deploy-%s.tar.zst", e2e.Arch))

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pkgPath, "--connected", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Verify the deployment does not have a mutated pod
	c, err := cluster.New(t.Context())
	require.NoError(t, err)

	deployment, err := c.Clientset.AppsV1().Deployments("connected-test").Get(t.Context(), "connected-deploy-test", metav1.GetOptions{})
	require.NoError(t, err)

	require.Equal(t, "ignore", deployment.Spec.Template.Labels[cluster.AgentLabel], "pod template should have zarf.dev/agent: ignore label")
	require.Equal(t, "ghcr.io/zarf-dev/doom-game:0.0.1", deployment.Spec.Template.Spec.Containers[0].Image, "image should not be rewritten in connected mode")

	deployedPkg, err := c.GetDeployedPackage(t.Context(), "connected-deploy")
	require.NoError(t, err)
	require.Equal(t, state.DeployModeConnected, deployedPkg.DeployMode, "package secret should record connected deploy mode")

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "connected-deploy", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}

func TestDevDeploy(t *testing.T) {
	// Generic test of dev deploy
	stdOut, stdErr, err := e2e.Zarf(t, "dev", "deploy", filepath.Join("src", "test", "packages", "47-connected-deploy"))
	require.NoError(t, err, stdOut, stdErr)

	c, err := cluster.New(t.Context())
	require.NoError(t, err)

	deployment, err := c.Clientset.AppsV1().Deployments("connected-test").Get(t.Context(), "connected-deploy-test", metav1.GetOptions{})
	require.NoError(t, err)

	require.Equal(t, "ignore", deployment.Spec.Template.Labels[cluster.AgentLabel], "pod template should have zarf.dev/agent: ignore label")
	require.Equal(t, "ghcr.io/zarf-dev/doom-game:0.0.1", deployment.Spec.Template.Spec.Containers[0].Image, "image should not be rewritten in connected mode")

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "kubectl", "delete", "namespace", "connected-test")
	require.NoError(t, err, stdOut, stdErr)

	// Special test of hidden registry-url flag
	stdOut, stdErr, err = e2e.Zarf(t, "dev", "deploy", "src/test/packages/99-registry-url", "--registry-url", "ghcr.io")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Zarf(t, "tools", "kubectl", "delete", "namespace", "registry-url")
	require.NoError(t, err, stdOut, stdErr)
}

func TestYOLOMode(t *testing.T) {
	t.Log("E2E: YOLO Mode")

	tmpdir := t.TempDir()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "examples/yolo", "-o", tmpdir)
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

	endpoints := tunnel.HTTPEndpoints()
	require.Len(t, endpoints, 1)

	// Check that 'curl' returns something.
	resp, err := http.Get(endpoints[0])
	require.NoError(t, err, resp)
	require.Equal(t, 200, resp.StatusCode)

	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "yolo", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
