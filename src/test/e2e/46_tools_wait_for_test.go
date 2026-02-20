// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"
	"time"

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
		t.Parallel()
		stdout, stderr, err := e2e.Zarf(t, "tools", "wait-for", "pod", "does-not-exist-pod", "ready", "-n", namespace, "--timeout", "3s")
		require.Error(t, err, stdout, stderr)
	})

	t.Run("wait for resource without specifying the namespace only looks in default namespace", func(t *testing.T) {
		t.Parallel()
		// There are never any jobs by default
		_, _, err := e2e.Zarf(t, "tools", "wait-for", "jobs", "--timeout", "3s")
		require.Error(t, err)
	})

	t.Run("wait for resource pulls from default namespace", func(t *testing.T) {
		t.Parallel()
		// There's always a kubernetes svc in the default namespace
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "svc", "--timeout", "3s")
		require.NoError(t, err, stdOut, stdErr)
		stdOut, stdErr, err = e2e.Zarf(t, "tools", "wait-for", "resource", "svc", "--timeout", "3s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait for existing resource succeeds immediately", func(t *testing.T) {
		t.Parallel()
		podName := "existing-pod"

		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err = e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "po", podName, "ready", "-n", namespace, "--timeout", "20s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait for resource existence", func(t *testing.T) {
		t.Parallel()
		configMapName := "exists-test-cm"

		_, _, err := e2e.Kubectl(t, "create", "configmap", configMapName, "-n", namespace)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err = e2e.Kubectl(t, "delete", "configmap", configMapName, "-n", namespace)
			require.NoError(t, err)
		})

		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "configmap", configMapName, "exists", "-n", namespace, "--timeout", "30s")
		require.NoError(t, err, stdOut, stdErr)
		stdOut, stdErr, err = e2e.Zarf(t, "tools", "wait-for", "configmap", configMapName, "create", "-n", namespace, "--timeout", "30s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait for delete succeeds on non-existent resource", func(t *testing.T) {
		t.Parallel()
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "configmap", "does-not-exist", "delete", "-n", namespace, "--timeout", "10s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait with label selector", func(t *testing.T) {
		t.Parallel()
		podName := "labeled-pod"

		// Create a pod with a specific label
		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--labels=test-label=wait-test", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		// Wait using label selector
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "pod", "test-label=wait-test", "ready", "-n", namespace, "--timeout", "20s")
		require.NoError(t, err, stdOut, stdErr)
		stdOut, stdErr, err = e2e.Zarf(t, "tools", "wait-for", "resource", "pod", "test-label=wait-test", "ready", "-n", namespace, "--timeout", "20s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait with jsonpath condition", func(t *testing.T) {
		t.Parallel()
		podName := "jsonpath-pod"

		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "pod", podName, "{.status.phase}=Running", "-n", namespace, "--timeout", "20s")
		require.NoError(t, err, stdOut, stdErr)
		stdOut, stdErr, err = e2e.Zarf(t, "tools", "wait-for", "pod", podName, "'{.status.phase}'=Running", "-n", namespace, "--timeout", "20s")
		require.NoError(t, err, stdOut, stdErr)
		// Advanced condition
		stdOut, stdErr, err = e2e.Zarf(t, "tools", "wait-for", "pod", podName, "{.status.conditions[?(@.type==\"ContainersReady\")].status}=True", "-n", namespace, "--timeout", "20s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait for any resource of kind times out when none exist", func(t *testing.T) {
		t.Parallel()
		// Create a fresh namespace with no deployments
		emptyNamespace := "wait-for-empty"
		_, _, err := e2e.Kubectl(t, "create", "namespace", emptyNamespace)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "namespace", emptyNamespace, "--force=true", "--wait=false", "--grace-period=0")
			require.NoError(t, err)
		})

		// Wait for any deployment in the empty namespace - should timeout
		_, _, err = e2e.Zarf(t, "tools", "wait-for", "deployment", "-n", emptyNamespace, "--timeout", "3s")
		require.Error(t, err)
	})

	t.Run("wait for any resource of kind succeeds when one exists", func(t *testing.T) {
		t.Parallel()
		// Create a configmap in the namespace
		_, _, err := e2e.Kubectl(t, "create", "configmap", "any-kind-test-cm", "-n", namespace)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "configmap", "any-kind-test-cm", "-n", namespace)
			require.NoError(t, err)
		})

		// Wait for any configmap in the namespace - should succeed
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "configmap", "-n", namespace, "--timeout", "10s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait for any cluster-scoped resource of kind", func(t *testing.T) {
		t.Parallel()
		stdOut, stdErr, err := e2e.Zarf(t, "tools", "wait-for", "storageclass", "--timeout", "10s")
		require.NoError(t, err, stdOut, stdErr)
	})

	t.Run("wait for CRD and CR that do not exist in the cluster when wait begins", func(t *testing.T) {
		t.Parallel()
		crdName := "zarfwaittests.test.zarf.dev"
		resourceName := "my-wait-test"

		crdFile := "src/test/packages/46-manifests/zarf-crd.yaml"
		resourceFile := "src/test/packages/46-manifests/zarf-cr.yaml"

		// Start waiting before the CRD exists
		errCh := make(chan error, 1)
		go func() {
			_, _, err := e2e.Zarf(t, "tools", "wait-for", "ZarfWaitTest", resourceName, "exists", "-n", namespace, "--timeout", "30s")
			errCh <- err
		}()

		// Let the wait start and fail to resolve the resource kind
		time.Sleep(3 * time.Second)

		_, _, err := e2e.Kubectl(t, "apply", "-f", crdFile)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "-f", crdFile)
			require.NoError(t, err)
		})

		// Wait for the CRD to be established before creating an instance
		_, _, err = e2e.Zarf(t, "tools", "wait-for", "crds", crdName, "established", "--timeout=10s")
		require.NoError(t, err)

		_, _, err = e2e.Kubectl(t, "apply", "-f", resourceFile)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "-f", resourceFile)
			require.NoError(t, err)
		})

		// The wait should succeed now that the CRD and resource exist
		err = <-errCh
		require.NoError(t, err)
	})

	t.Run("wait for CRD with kind name that conflicts with a built-in resource", func(t *testing.T) {
		t.Parallel()
		crdName := "services.test.zarf.dev"
		resourceName := "my-svc-test"

		crdFile := "src/test/packages/46-manifests/zarf-svc-crd.yaml"
		resourceFile := "src/test/packages/46-manifests/zarf-svc-cr.yaml"

		_, _, err := e2e.Kubectl(t, "apply", "-f", crdFile)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "-f", crdFile)
			require.NoError(t, err)
		})

		stdout, stderr, err := e2e.Zarf(t, "tools", "wait-for", "crds", crdName, "established", "--timeout=10s")
		require.NoError(t, err, stdout, stderr)

		_, _, err = e2e.Kubectl(t, "apply", "-f", resourceFile)
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "-f", resourceFile)
			require.NoError(t, err)
		})

		// "Service" also matches the built-in k8s Service kind
		stdout, stderr, err = e2e.Zarf(t, "tools", "wait-for", "services.test.zarf.dev", resourceName, "exists", "-n", namespace, "--timeout", "20s")
		require.NoError(t, err, stdout, stderr)
	})

	t.Run("wait for pod created after wait starts", func(t *testing.T) {
		t.Parallel()
		podName := "delayed-pod"

		// Start waiting for the pod in a goroutine before it exists
		errCh := make(chan error, 1)
		go func() {
			_, _, err := e2e.Zarf(t, "tools", "wait-for", "pod", podName, "ready", "-n", namespace, "--timeout", "20s")
			errCh <- err
		}()

		// Let the wait attempt to pull the pod
		time.Sleep(3 * time.Second)

		// Create the pod after the wait has started
		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		// Wait should succeed after the pod is created and becomes ready
		err = <-errCh
		require.NoError(t, err)
	})

	t.Run("wait for resource readiness automatically", func(t *testing.T) {
		t.Parallel()
		podName := "pod-readiness"

		// Start waiting for the pod in a goroutine before it exists
		errCh := make(chan error, 1)
		go func() {
			_, _, err := e2e.Zarf(t, "tools", "wait-for", "resource", "pod", podName, "-n", namespace, "--timeout", "20s")
			errCh <- err
		}()

		// Let the wait attempt to pull the pod
		time.Sleep(3 * time.Second)

		// Create the pod after the wait has started
		_, _, err := e2e.Kubectl(t, "run", podName, "-n", namespace, "--image=busybox:latest", "--restart=Never", "--", "sleep", "300")
		require.NoError(t, err)

		t.Cleanup(func() {
			_, _, err := e2e.Kubectl(t, "delete", "pod", podName, "-n", namespace, "--force=true", "--grace-period=0")
			require.NoError(t, err)
		})

		// Wait should succeed after the pod is created and becomes ready
		err = <-errCh
		require.NoError(t, err)
	})
}
