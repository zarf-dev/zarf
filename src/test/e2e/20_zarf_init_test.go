// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
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

	// run `zarf init`
	_, stdErr, err := exec.CmdWithContext(ctx, exec.PrintCfg(), e2e.ZarfBinPath, "init", "--components="+initComponents, "--confirm", "--nodeport", "31337")
	require.Contains(t, stdErr, "artifacts with software bill-of-materials (SBOM) included")
	require.NoError(t, err)

	// Check that gitea is actually running and healthy
	stdOut, _, err := e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-l", "app in (gitea)", "-n", "zarf", "-o", "jsonpath={.items[*].status.phase}")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Running")

	// Check that the logging stack is actually running and healthy
	stdOut, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-l", "app in (loki)", "-n", "zarf", "-o", "jsonpath={.items[*].status.phase}")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Running")
	stdOut, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-l", "app.kubernetes.io/name in (grafana)", "-n", "zarf", "-o", "jsonpath={.items[*].status.phase}")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Running")
	stdOut, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "pods", "-l", "app.kubernetes.io/name in (promtail)", "-n", "zarf", "-o", "jsonpath={.items[*].status.phase}")
	require.NoError(t, err)
	require.Contains(t, stdOut, "Running")

	// Check that the registry is running on the correct NodePort
	stdOut, _, err = e2e.ExecZarfCommand("tools", "kubectl", "get", "service", "-n", "zarf", "zarf-docker-registry", "-o=jsonpath='{.spec.ports[*].nodePort}'")
	require.NoError(t, err)
	require.Contains(t, stdOut, "31337")

	// Special sizing-hacking for reducing resources where Kind + CI eats a lot of free cycles (ignore errors)
	_, _, _ = e2e.ExecZarfCommand("tools", "kubectl", "scale", "deploy", "-n", "kube-system", "coredns", "--replicas=1")
	_, _, _ = e2e.ExecZarfCommand("tools", "kubectl", "scale", "deploy", "-n", "zarf", "agent-hook", "--replicas=1")
}
