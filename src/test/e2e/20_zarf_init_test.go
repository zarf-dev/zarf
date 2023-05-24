// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/base64"
	"fmt"
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

	t.Run("init mismatched arch", func(t *testing.T) {
		t.Parallel()
		var (
			mismatchedArch        = e2e.GetMismatchedArch()
			initPackageVersion    = "UnknownVersion"
			mismatchedInitPackage = fmt.Sprintf("zarf-init-%s-%s.tar.zst", mismatchedArch, initPackageVersion)
			expectedErrorMessage  = fmt.Sprintf("this package architecture is %s", mismatchedArch)
		)

		// Build init package with different arch than the cluster arch.
		stdOut, stdErr, err := e2e.ZarfWithConfirm("package", "create", ".", "--architecture", mismatchedArch)
		require.NoError(t, err, stdOut, stdErr)
		t.Cleanup(func() {
			e2e.CleanFiles(mismatchedInitPackage)
		})
		// Check that `zarf init` fails in appliance mode when we try to initialize a k3s cluster
		// on a machine with a different architecture than the package architecture.
		// We need to use the --architecture flag here to force zarf to find the package.
		componentsFlag := ""
		if e2e.ApplianceMode {
			componentsFlag = "--components=k3s"
		}
		_, stdErr, err = e2e.ZarfWithConfirm("init", "--architecture", mismatchedArch, componentsFlag)
		require.Error(t, err, stdErr)
		require.Contains(t, stdErr, expectedErrorMessage)
	})

	t.Run("init with components and verify state secrets are sanitzed", func(t *testing.T) {
		t.Parallel()
		// run `zarf init`
		_, initStdErr, err := e2e.ZarfWithConfirm("init", "--components="+initComponents, "--nodeport", "31337", "-l", "trace")
		require.NoError(t, err)
		require.Contains(t, initStdErr, "artifacts with software bill-of-materials (SBOM) included")

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

		// Check that the registry is running on the correct NodePort
		stdOut, _, err := e2e.Kubectl("get", "service", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.ports[*].nodePort}'")
		require.NoError(t, err)
		require.Contains(t, stdOut, "31337")

		// Check that the registry is running with the correct scale down policy
		stdOut, _, err = e2e.Kubectl("get", "hpa", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.behavior.scaleDown.selectPolicy}'")
		require.NoError(t, err)
		require.Contains(t, stdOut, "Min")

		// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
		_, _, _ = e2e.Kubectl("scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1")
		_, _, _ = e2e.Kubectl("scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")
	})
}
