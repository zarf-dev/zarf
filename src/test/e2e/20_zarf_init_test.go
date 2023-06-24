// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/base64"
	"fmt"
	"strings"
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

	// Get the version of the CLI
	stdOut, stdErr, err := e2e.Zarf("version")
	require.NoError(t, err, stdOut, stdErr)
	initPackageVersion := strings.Trim(stdOut, "\n")

	var (
		mismatchedArch        = e2e.GetMismatchedArch()
		mismatchedInitPackage = fmt.Sprintf("zarf-init-%s-%s.tar.zst", mismatchedArch, initPackageVersion)
		expectedErrorMessage  = fmt.Sprintf("this package architecture is %s", mismatchedArch)
	)
	t.Cleanup(func() {
		e2e.CleanFiles(mismatchedInitPackage)
	})

	// Build init package with different arch than the cluster arch.
	stdOut, stdErr, err = e2e.Zarf("package", "create", "src/test/packages/20-mismatched-arch-init", "--architecture", mismatchedArch, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	// Check that `zarf init` returns an error because of the mismatched architectures.
	// We need to use the --architecture flag here to force zarf to find the package.
	componentsFlag := ""
	if e2e.ApplianceMode {
		// make sure init fails in appliance mode when we try to initialize a k3s cluster
		// with behavior from the k3s component's actions
		componentsFlag = "--components=k3s"
	}
	_, stdErr, err = e2e.Zarf("init", "--architecture", mismatchedArch, componentsFlag, "--confirm")
	require.Error(t, err, stdErr)
	require.Contains(t, stdErr, expectedErrorMessage)

	if !e2e.ApplianceMode {
		// throw a pending pod into the cluster to ensure we can properly ignore them when selecting images
		_, _, err = e2e.Kubectl("apply", "-f", "https://raw.githubusercontent.com/kubernetes/website/main/content/en/examples/pods/pod-with-node-affinity.yaml")
		require.NoError(t, err)
	}

	// run `zarf init`
	_, initStdErr, err := e2e.Zarf("init", "--components="+initComponents, "--nodeport", "31337", "-l", "trace", "--confirm")
	require.NoError(t, err)
	require.Contains(t, initStdErr, "an inventory of all software contained in this package")

	logText := e2e.GetLogFileContents(t, initStdErr)

	// Verify that any state secrets were not included in the log
	base64State, _, err := e2e.Kubectl("get", "secret", "zarf-state", "-n", "zarf", "-o", "jsonpath={.data.state}")
	require.NoError(t, err)
	stateJSON, err := base64.StdEncoding.DecodeString(base64State)
	require.NoError(t, err)
	state := types.ZarfState{}
	err = json.Unmarshal(stateJSON, &state)
	require.NoError(t, err)
	require.NotContains(t, logText, state.AgentTLS.CA)
	require.NotContains(t, logText, state.AgentTLS.Cert)
	require.NotContains(t, logText, state.AgentTLS.Key)
	require.NotContains(t, logText, state.ArtifactServer.PushToken)
	require.NotContains(t, logText, state.GitServer.PullPassword)
	require.NotContains(t, logText, state.GitServer.PushPassword)
	require.NotContains(t, logText, state.RegistryInfo.PullPassword)
	require.NotContains(t, logText, state.RegistryInfo.PushPassword)
	require.NotContains(t, logText, state.RegistryInfo.Secret)
	require.NotContains(t, logText, state.LoggingSecret)

	if e2e.ApplianceMode {
		// make sure that we upgraded `k3s` correctly and are running the correct version - this should match that found in `packages/distros/k3s`
		kubeletVersion, _, err := e2e.Kubectl("get", "nodes", "-o", "jsonpath={.items[0].status.nodeInfo.kubeletVersion}")
		require.NoError(t, err)
		require.Contains(t, kubeletVersion, "v1.27.2+k3s1")
	}

	// Check that the registry is running on the correct NodePort
	stdOut, _, err = e2e.Kubectl("get", "service", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.ports[*].nodePort}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "31337")

	// Check that the registry is running with the correct scale down policy
	stdOut, _, err = e2e.Kubectl("get", "hpa", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.behavior.scaleDown.selectPolicy}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Min")

	// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
	_, _, _ = e2e.Kubectl("scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1")
	_, _, _ = e2e.Kubectl("scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")
}
