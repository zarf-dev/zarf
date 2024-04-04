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

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestZarfInit(t *testing.T) {
	t.Log("E2E: Zarf init")
	e2e.SetupWithCluster(t)

	initComponents := "logging,git-server"
	// Add k3s component in appliance mode
	if e2e.ApplianceMode {
		initComponents = "k3s,logging,git-server"
	}

	initPackageVersion := e2e.GetZarfVersion(t)

	var (
		mismatchedArch        = e2e.GetMismatchedArch()
		mismatchedInitPackage = fmt.Sprintf("zarf-init-%s-%s.tar.zst", mismatchedArch, initPackageVersion)
		expectedErrorMessage  = "unable to run component before action: command \"Check that the host architecture matches the package architecture\""
	)
	t.Cleanup(func() {
		e2e.CleanFiles(mismatchedInitPackage)
	})

	if runtime.GOOS == "linux" {
		// Build init package with different arch than the cluster arch.
		stdOut, stdErr, err := e2e.Zarf("package", "create", "src/test/packages/20-mismatched-arch-init", "--architecture", mismatchedArch, "--confirm")
		require.NoError(t, err, stdOut, stdErr)

		// Check that `zarf init` returns an error because of the mismatched architectures.
		// We need to use the --architecture flag here to force zarf to find the package.
		_, stdErr, err = e2e.Zarf("init", "--architecture", mismatchedArch, "--components=k3s", "--confirm")
		require.Error(t, err, stdErr)
		require.Contains(t, stdErr, expectedErrorMessage)
	}

	if !e2e.ApplianceMode {
		// throw a pending pod into the cluster to ensure we can properly ignore them when selecting images
		_, _, err := e2e.Kubectl("apply", "-f", "https://raw.githubusercontent.com/kubernetes/website/main/content/en/examples/pods/pod-with-node-affinity.yaml")
		require.NoError(t, err)
	}

	// Check for any old secrets to ensure that they don't get saved in the init log
	oldState := types.ZarfState{}
	base64State, _, err := e2e.Kubectl("get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	if err == nil {
		oldStateJSON, err := base64.StdEncoding.DecodeString(base64State)
		require.NoError(t, err)
		err = json.Unmarshal(oldStateJSON, &oldState)
		require.NoError(t, err)
	}

	// run `zarf init`
	_, initStdErr, err := e2e.Zarf("init", "--components="+initComponents, "--nodeport", "31337", "-l", "trace", "--confirm")
	require.NoError(t, err)
	require.Contains(t, initStdErr, "an inventory of all software contained in this package")
	require.NotContains(t, initStdErr, "This package does NOT contain an SBOM. If you require an SBOM, please contact the creator of this package to request a version that includes an SBOM.")

	logText := e2e.GetLogFileContents(t, e2e.StripMessageFormatting(initStdErr))

	// Verify that any state secrets were not included in the log
	state := types.ZarfState{}
	base64State, _, err = e2e.Kubectl("get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	require.NoError(t, err)
	stateJSON, err := base64.StdEncoding.DecodeString(base64State)
	require.NoError(t, err)
	err = json.Unmarshal(stateJSON, &state)
	require.NoError(t, err)
	checkLogForSensitiveState(t, logText, state)

	// Check the old state values as well (if they exist) to ensure they weren't printed and then updated during init
	if oldState.LoggingSecret != "" {
		checkLogForSensitiveState(t, logText, oldState)
	}

	if e2e.ApplianceMode {
		// make sure that we upgraded `k3s` correctly and are running the correct version - this should match that found in `packages/distros/k3s`
		kubeletVersion, _, err := e2e.Kubectl("get", "nodes", "-o", "jsonpath={.items[0].status.nodeInfo.kubeletVersion}")
		require.NoError(t, err)
		require.Contains(t, kubeletVersion, "v1.28.4+k3s2")
	}

	// Check that the registry is running on the correct NodePort
	stdOut, _, err := e2e.Kubectl("get", "service", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.ports[*].nodePort}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "31337")

	// Check that the registry is running with the correct scale down policy
	stdOut, _, err = e2e.Kubectl("get", "hpa", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.behavior.scaleDown.selectPolicy}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Min")

	verifyZarfNamespaceLabels(t)
	verifyZarfSecretLabels(t)
	verifyZarfPodLabels(t)
	verifyZarfServiceLabels(t)

	// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
	_, _, _ = e2e.Kubectl("scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1")
	_, _, _ = e2e.Kubectl("scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")
}

func checkLogForSensitiveState(t *testing.T, logText string, zarfState types.ZarfState) {
	t.Helper()

	require.NotContains(t, logText, zarfState.AgentTLS.CA)
	require.NotContains(t, logText, string(zarfState.AgentTLS.CA))
	require.NotContains(t, logText, zarfState.AgentTLS.Cert)
	require.NotContains(t, logText, string(zarfState.AgentTLS.Cert))
	require.NotContains(t, logText, zarfState.AgentTLS.Key)
	require.NotContains(t, logText, string(zarfState.AgentTLS.Key))
	require.NotContains(t, logText, zarfState.ArtifactServer.PushToken)
	require.NotContains(t, logText, zarfState.GitServer.PullPassword)
	require.NotContains(t, logText, zarfState.GitServer.PushPassword)
	require.NotContains(t, logText, zarfState.RegistryInfo.PullPassword)
	require.NotContains(t, logText, zarfState.RegistryInfo.PushPassword)
	require.NotContains(t, logText, zarfState.RegistryInfo.Secret)
	require.NotContains(t, logText, zarfState.LoggingSecret)
}

func verifyZarfNamespaceLabels(t *testing.T) {
	t.Helper()

	expectedLabels := `'{"app.kubernetes.io/managed-by":"zarf","kubernetes.io/metadata.name":"zarf"}'`
	actualLabels, _, err := e2e.Kubectl("get", "ns", "zarf", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}

func verifyZarfSecretLabels(t *testing.T) {
	t.Helper()

	// zarf state
	expectedLabels := `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err := e2e.Kubectl("get", "-n=zarf", "secret", "zarf-state", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// init package secret
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf","package-deploy-info":"init"}'`
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "secret", "zarf-package-init", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// registry
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "secret", "private-registry", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// agent hook TLS
	//
	// this secret does not have the managed by zarf label
	// because it is deployed as a helm chart rather than generated in Go code.
	expectedLabels = `'{"app.kubernetes.io/managed-by":"Helm"}'`
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "secret", "agent-hook-tls", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// git server
	expectedLabels = `'{"app.kubernetes.io/managed-by":"zarf"}'`
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "secret", "private-git-server", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}

func verifyZarfPodLabels(t *testing.T) {
	t.Helper()

	// registry
	podHash, _, err := e2e.Kubectl("get", "-n=zarf", "--selector=app=docker-registry", "pods", `-o=jsonpath="{.items[0].metadata.labels['pod-template-hash']}"`)
	require.NoError(t, err)
	expectedLabels := fmt.Sprintf(`'{"app":"docker-registry","pod-template-hash":%s,"release":"zarf-docker-registry","zarf.dev/agent":"ignore"}'`, podHash)
	actualLabels, _, err := e2e.Kubectl("get", "-n=zarf", "--selector=app=docker-registry", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// agent
	podHash, _, err = e2e.Kubectl("get", "-n=zarf", "--selector=app=agent-hook", "pods", `-o=jsonpath="{.items[0].metadata.labels['pod-template-hash']}"`)
	require.NoError(t, err)
	expectedLabels = fmt.Sprintf(`'{"app":"agent-hook","pod-template-hash":%s,"zarf.dev/agent":"ignore"}'`, podHash)
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "--selector=app=agent-hook", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// logging and git server pods should have the `zarf-agent=patched` label
	// since they should have been mutated by the agent
	patchedLabel := `"zarf-agent":"patched"`

	// logging
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "--selector=app.kubernetes.io/instance=zarf-loki-stack", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Contains(t, actualLabels, patchedLabel)

	// git server
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "--selector=app.kubernetes.io/instance=zarf-gitea  ", "pods", "-o=jsonpath='{.items[0].metadata.labels}'")
	require.NoError(t, err)
	require.Contains(t, actualLabels, patchedLabel)
}

func verifyZarfServiceLabels(t *testing.T) {
	t.Helper()

	// registry
	expectedLabels := `'{"app.kubernetes.io/managed-by":"Helm","zarf.dev/connect-name":"registry"}'`
	actualLabels, _, err := e2e.Kubectl("get", "-n=zarf", "service", "zarf-connect-registry", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// logging
	expectedLabels = `'{"app.kubernetes.io/managed-by":"Helm","zarf.dev/connect-name":"logging"}'`
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "service", "zarf-connect-logging", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)

	// git server
	expectedLabels = `'{"app.kubernetes.io/managed-by":"Helm","zarf.dev/connect-name":"git"}'`
	actualLabels, _, err = e2e.Kubectl("get", "-n=zarf", "service", "zarf-connect-git", "-o=jsonpath='{.metadata.labels}'")
	require.NoError(t, err)
	require.Equal(t, expectedLabels, actualLabels)
}
