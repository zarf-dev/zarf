// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComponentWebhooks(t *testing.T) {
	t.Log("E2E: Component Webhooks")

	// Deploy example Pepr webhook.
	webhookPath := fmt.Sprintf("build/zarf-package-component-webhooks-%s-0.0.1.tar.zst", e2e.Arch)
	stdOut, stdErr, err := e2e.Zarf(t, "package", "deploy", webhookPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	stdOut, stdErr, err = e2e.Zarf(t, "tools", "wait-for", "deployment", "pepr-cb5693ef-d13c-5fe1-b5ad-c870fd911b3b", "available", "-n=pepr-system")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(webhookPath)

	// Ensure package deployments wait for webhooks to complete.
	gamesPath := fmt.Sprintf("build/zarf-package-dos-games-%s-1.1.0.tar.zst", e2e.Arch)
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", gamesPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Waiting for webhook \"test-webhook\" to complete for component \"baseline\"")

	// Ensure package deployments with the '--skip-webhooks' flag do not wait on webhooks to complete.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", gamesPath, "--skip-webhooks", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.NotContains(t, stdErr, "Waiting for webhook \"test-webhook\" to complete for component \"baseline\"")

	// Remove the Pepr webhook package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "component-webhooks", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	stdOut, stdErr, err = e2e.Kubectl(t, "delete", "namespace", "pepr-system")
	require.NoError(t, err, stdOut, stdErr)

	// Remove the dos-games package.
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	stdOut, stdErr, err = e2e.Kubectl(t, "delete", "namespace", "dos-games")
	require.NoError(t, err, stdOut, stdErr)
}
