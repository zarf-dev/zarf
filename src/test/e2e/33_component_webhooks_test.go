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
	e2e.SetupWithCluster(t)

	// Deploy example Pepr capability
	webhookPath := fmt.Sprintf("build/zarf-package-component-webhooks-%s-0.0.1.tar.zst", e2e.Arch)
	stdOut, stdErr, err := e2e.Zarf("package", "deploy", webhookPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Deploy dos-games package and ensure it waits for the Pepr webhook to complete.
	gamesPath := fmt.Sprintf("build/zarf-package-dos-games-%s-1.0.0.tar.zst", e2e.Arch)
	stdOut, stdErr, err = e2e.Zarf("package", "deploy", gamesPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdErr, "Waiting for component webhooks to complete")

	// Remove the Pepr webhook package.
	stdOut, stdErr, err = e2e.Zarf("package", "remove", "component-webhooks", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.Kubectl("delete", "namespace", "pepr-system")
	require.NoError(t, err, stdOut, stdErr)
}
