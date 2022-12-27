// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for zarf
package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/stretchr/testify/require"
)

func TestCosignDeploy(t *testing.T) {
	t.Log("E2E: Cosign deploy")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	// Test with command from https://zarf.dev/install/
	command := fmt.Sprintf("%s package deploy sget://defenseunicorns/zarf-hello-world:$(uname -m) --confirm", e2e.zarfBinPath)

	stdOut, stdErr, err := exec.CmdWithContext(context.TODO(), exec.WithPrint(), "sh", "-c", command)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "dos-games", "--confirm")
	require.NoError(t, err, stdOut, stdErr)
}
