// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
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
