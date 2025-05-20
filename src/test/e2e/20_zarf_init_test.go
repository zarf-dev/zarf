// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/base64"
	"fmt"
	"runtime"
	"testing"

	"encoding/json"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/types"
)

func TestZarfInit(t *testing.T) {
	t.Log("E2E: Zarf init")

	initComponents := "git-server"
	if e2e.ApplianceMode {
		initComponents = "k3s,git-server"
	}

	initPackageVersion := e2e.GetZarfVersion(t)

	var (
		mismatchedArch        = e2e.GetMismatchedArch()
		mismatchedInitPackage = fmt.Sprintf("zarf-init-%s-%s.tar.zst", mismatchedArch, initPackageVersion)
		expectedErrorMessage  = "unable to run component before action: command \"Check that the host architecture matches the package architecture\""
	)
	t.Cleanup(func() {
		e2e.CleanFiles(t, mismatchedInitPackage)
	})

	if runtime.GOOS == "linux" {
		// Build init package with different arch than the cluster arch.
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/20-mismatched-arch-init", "--architecture", mismatchedArch, "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		// Check that `zarf init` returns an error because of the mismatched architectures.
		// We need to use the --architecture flag here to force zarf to find the package.
		_, stdErr, err = e2e.Zarf(t, "init", "--architecture", mismatchedArch, "--components=k3s", "--confirm")
		require.Error(t, err, stdErr)
		require.Contains(t, stdErr, expectedErrorMessage)
	}

	if !e2e.ApplianceMode {
		// throw a pending pod into the cluster to ensure we can properly ignore them when selecting images
		_, _, err := e2e.Kubectl(t, "apply", "-f", "https://raw.githubusercontent.com/kubernetes/website/main/content/en/examples/pods/pod-with-node-affinity.yaml")
		require.NoError(t, err)
	}

	// Check for any old secrets to ensure that they don't get saved in the init log
	oldState := types.ZarfState{}
	base64State, _, err := e2e.Kubectl(t, "get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	if err == nil {
		oldStateJSON, err := base64.StdEncoding.DecodeString(base64State)
		require.NoError(t, err)
		err = json.Unmarshal(oldStateJSON, &oldState)
		require.NoError(t, err)
	}

	// run `zarf init`
	_, _, err = e2e.Zarf(t, "init", "--components="+initComponents, "--nodeport", "31337", "--confirm")
	require.NoError(t, err)

	// Verify that any state secrets were not included in the log
	state := types.ZarfState{}
	base64State, _, err = e2e.Kubectl(t, "get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	require.NoError(t, err)
	stateJSON, err := base64.StdEncoding.DecodeString(base64State)
	require.NoError(t, err)
	err = json.Unmarshal(stateJSON, &state)
	require.NoError(t, err)

	if e2e.ApplianceMode {
		// make sure that we upgraded `k3s` correctly and are running the correct version - this should match that found in `packages/distros/k3s`
		kubeletVersion, _, err := e2e.Kubectl(t, "get", "nodes", "-o", "jsonpath={.items[0].status.nodeInfo.kubeletVersion}")
		require.NoError(t, err)
		require.Contains(t, kubeletVersion, "v1.29.10+k3s1")
	}

	// Check that the registry is running on the correct NodePort
	stdOut, _, err := e2e.Kubectl(t, "get", "service", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.ports[*].nodePort}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "31337")

	// Check that the registry is running with the correct scale down policy
	stdOut, _, err = e2e.Kubectl(t, "get", "hpa", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.behavior.scaleDown.selectPolicy}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Min")

	verifyZarfNamespaceLabels(t)
	verifyZarfSecretLabels(t)
	verifyZarfPodLabels(t)
	verifyZarfServiceLabels(t)

	// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
	_, _, _ = e2e.Kubectl(t, "scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1") //nolint:errcheck
	_, _, _ = e2e.Kubectl(t, "scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")     //nolint:errcheck
}

func verifyZarfNamespaceLabels(t *testing.T) {
	t.Helper()

	expectedLabels := `'{"app.kubernetes.io/managed-by":"zarf","kubernetes.io/metadata.name":"zarf"}'`
	actualLabels, _, err := e2e.Kubectl(t, "get", "ns", "zarf", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}

func verifyZarfSecretLabels(t *testing.T) {
	t.Helper()

	// zarf state
	expectedLabels := `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err := e2e.Kubectl(t, "get", "-n=zarf", "secret", "zarf-state", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// init package secret
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf","package-deploy-info":"init"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "secret", "zarf-package-init", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// registry
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "secret", "private-registry", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// agent hook TLS
	//
	// this secret does not have the managed by zarf label
	// because it is deployed as a helm chart rather than generated in Go code.
	expectedLabels = `'{"app.kubernetes.io/managed-by":"Helm"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "secret", "agent-hook-tls", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// git server
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "secret", "private-git-server", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}

func verifyZarfPodLabels(t *testing.T) {
	t.Helper()

	// registry
	podHash, _, err := e2e.Kubectl(t, "get", "-n=zarf", "--selector=app=docker-registry", "pods", `-o=jsonpath="{.items[0].metadata.labels['pod-template-hash']}"`)
	require.NoError(t, err)
	expectedLabels := fmt.Sprintf(`'{"app":"docker-registry","pod-template-hash":%s,"release":"zarf-docker-registry","zarf.dev/agent":"ignore"}'`, podHash)
	actualLabels, _, err := e2e.Kubectl(t, "get", "-n=zarf", "--selector=app=docker-registry", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// agent
	podHash, _, err = e2e.Kubectl(t, "get", "-n=zarf", "--selector=app=agent-hook", "pods", `-o=jsonpath="{.items[0].metadata.labels['pod-template-hash']}"`)
	require.NoError(t, err)
	expectedLabels = fmt.Sprintf(`'{"app":"agent-hook","pod-template-hash":%s,"zarf.dev/agent":"ignore"}'`, podHash)
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "--selector=app=agent-hook", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// git server
	patchedLabel := `"zarf-agent":"patched"`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "--selector=app.kubernetes.io/instance=zarf-gitea  ", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Contains(t, actualLabels, patchedLabel)
}

func verifyZarfServiceLabels(t *testing.T) {
	t.Helper()

	// registry
	expectedLabels := `'{"app.kubernetes.io/managed-by":"Helm","zarf.dev/connect-name":"registry"}'`
	actualLabels, _, err := e2e.Kubectl(t, "get", "-n=zarf", "service", "zarf-connect-registry", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// git server
	expectedLabels = `'{"app.kubernetes.io/managed-by":"Helm","zarf.dev/connect-name":"git"}'`
	actualLabels, _, err = e2e.Kubectl(t, "get", "-n=zarf", "service", "zarf-connect-git", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}
