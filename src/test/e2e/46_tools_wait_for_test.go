// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWaitFor(t *testing.T) {
	t.Log("E2E: zarf tools wait-for")

	namespace := "wait-for-test"
	_, _, err := e2e.Kubectl(t, "create", "namespace", namespace)
	require.NoError(t, err)
	_, _, err = e2e.Kubectl(t, "label", "namespace", namespace, "zarf.dev/agent=ignore")
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _, err = e2e.Kubectl(t, "delete", "namespace", namespace, "--force=true", "--wait=false", "--grace-period=0")
		require.NoError(t, err)
	})

	t.Run("wait for non-existent resource times out", func(t *testing.T) {
		_, _, err := e2e.Zarf(t, "tools", "wait-for", "pod", "does-not-exist-pod", "ready", "-n", namespace, "--timeout", "3s")
		require.Error(t, err)
	})

	t.Run("wait for existing resource succeeds immediately", func(t *testing.T) {
		podName := "existing-pod"

		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err = e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "pod", podName, "ready", "-n", namespace, "--timeout", "30s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait for resource existence (not condition)", func(t *testing.T) {
		podName := "exists-test-pod"

		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err = e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "pod", podName, "exists", "-n", namespace, "--timeout", "30s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait with label selector", func(t *testing.T) {
		podName := "labeled-pod"

		// Create a pod with a specific label
		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--labels=test-label=wait-test", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		// Wait using label selector
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "pod", "test-label=wait-test", "ready", "-n", namespace, "--timeout", "60s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait with jsonpath condition", func(t *testing.T) {
		podName := "jsonpath-pod"

		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "pod", podName, "{.status.phase}=Running", "-n", namespace, "--timeout", "60s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait for resource by by kind", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "storageclass", "--timeout", "10s")
		require.NoError(t, err, stdOut, stdErr)
	})
}
