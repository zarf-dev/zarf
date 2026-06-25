// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAgentMutationPolicy verifies that in default-ignore policy the agent does not mutate
// pods running in namespaces that have not been labeled zarf.dev/agent=mutate.
func TestAgentMutationPolicy(t *testing.T) {
	t.Log("E2E: Agent mutation policy")

	// Don't run this test in appliance mode
	if e2e.ApplianceMode {
		t.Skip("skipping test in appliance mode to avoid re-initializing k3s")
	}

	const nsName = "agent-mutation-test"
	const podName = "alpine-unmutated"

	t.Cleanup(func() {
		_, _, err := e2e.Kubectl(t, "delete", "pod", podName, "-n", nsName, "--force=true", "--grace-period=0", "--ignore-not-found")
		require.NoError(t, err)
		_, _, err = e2e.Kubectl(t, "delete", "namespace", nsName, "--ignore-not-found")
		require.NoError(t, err)
	})

	tmpdir := t.TempDir()

	initPackageVersion := e2e.GetZarfVersion(t)
	initPackageName := fmt.Sprintf("zarf-init-%s-%s.tar.zst", e2e.Arch, initPackageVersion)

	_, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/98-agent-only-init", "-o", tmpdir, "--skip-sbom")
	require.NoError(t, err, stdErr)

	initPackagePath := filepath.Join(tmpdir, initPackageName)

	_, stdErr, err = e2e.Zarf(t, "init", initPackagePath, "--agent-mutation-policy=labeled", "--confirm")
	require.NoError(t, err, stdErr)

	// Snapshot what the API server can route to right after init, while the webhook is first created.
	// This will let us check if the endpoint slices have an address and if .endpoints.address[x].conditions.ready: true
	_, _, err = e2e.Kubectl(t, "get", "endpointslices", "-n", "zarf", "-l", "kubernetes.io/service-name=agent-hook", "-o", "yaml")
	require.NoError(t, err)

	// Create and run a pod in the unlabeled ns
	_, _, err = e2e.Kubectl(t, "create", "namespace", nsName)
	require.NoError(t, err)

	_, _, err = e2e.Kubectl(t, "run", podName, "-n", nsName,
		"--image=ghcr.io/zarf-dev/images/alpine:3.21.3", "--restart=Never", "--", "sleep", "100")
	require.NoError(t, err)

	// The agent must not have rewritten the image — it should still point to the original registry.
	podImage, _, err := e2e.Kubectl(t, "get", "pod", podName, "-n", nsName, "-o", "jsonpath={.spec.containers[0].image}")
	require.NoError(t, err)
	require.Equal(t, "ghcr.io/zarf-dev/images/alpine:3.21.3", podImage)
}
