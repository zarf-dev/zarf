// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/stretchr/testify/require"
)

func TestZarfInit(t *testing.T) {
	t.Log("E2E: Zarf init (limit to 10 minutes)")
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)

	initComponents := "logging,git-server"
	// Add k3s component in appliance mode
	if e2e.ApplianceMode {
		initComponents = "k3s,logging,git-server"
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Minute)
	defer cancel()

	var (
		mismatchedArch        = e2e.SetMismatchedArch()
		initPackageVersion    = "UnknownVersion"
		mismatchedInitPackage = fmt.Sprintf("zarf-init-%s-%s.tar.zst", mismatchedArch, initPackageVersion)
	)

	// Build init package with different arch than the cluster arch.
	stdOut, stdErr, _, err := e2e.ExecZarfCommand("package", "create", ".", "--architecture", mismatchedArch, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(mismatchedInitPackage)

	// Check that `zarf init` fails in appliance mode when we try to initialize a k3s cluster
	// on a machine with a different architecture than the package architecture.
	// We need to use the --architecture flag here to force zarf to find the package.
	_, stdErr, _, err = e2e.ExecZarfCommand("init", "--architecture", mismatchedArch, "--components=k3s", "--confirm")
	require.Error(t, err, stdErr)

	// run `zarf init`
	_, stdErr, _, err = exec.CmdWithContext(ctx, exec.PrintCfg(), e2e.ZarfBinPath, "init", "--components="+initComponents, "--confirm", "--nodeport", "31337")
	require.Contains(t, stdErr, "artifacts with software bill-of-materials (SBOM) included")
	require.NoError(t, err)

	// Check that gitea is actually running and healthy
	stdOut, _, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-l", "app in (gitea)", "-n", "zarf", "-o", "jsonpath={.items[*].status.phase}")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Running")

	// Check that the logging stack is actually running and healthy
	stdOut, _, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-l", "app in (loki)", "-n", "zarf", "-o", "jsonpath={.items[*].status.phase}")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Running")
	stdOut, _, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-l", "app.kubernetes.io/name in (grafana)", "-n", "zarf", "-o", "jsonpath={.items[*].status.phase}")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Running")
	stdOut, _, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-l", "app.kubernetes.io/name in (promtail)", "-n", "zarf", "-o", "jsonpath={.items[*].status.phase}")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Running")

	// Check that the registry is running on the correct NodePort
	stdOut, _, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "service", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.ports[*].nodePort}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "31337")

	// Check that the registry is running with the correct scale down policy
	stdOut, _, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "hpa", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.behavior.scaleDown.selectPolicy}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Min")

	// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
	_, _, _, _ = e2e.ExecZarfCommand("tools", "kubectl", "scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1")
	_, _, _, _ = e2e.ExecZarfCommand("tools", "kubectl", "scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")
}
