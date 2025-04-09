// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEphemeralContainers(t *testing.T) {
	t.Log("E2E: Ephemeral Containers mutation")

	// cleanup - should perform cleanup in the event of pass or fail
	t.Cleanup(func() {
		e2e.Zarf(t, "package", "remove", "basic-pod", "--confirm") //nolint:errcheck
	})

	tmpdir := t.TempDir()

	// we need to create a test package that contains the images we want to potentially use
	// this should ideally be a single pod such that naming is static
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/38-ephemeral-container", "-o", tmpdir, "--skip-sbom")
	require.NoError(t, err, stdOut, stdErr)
	packageName := fmt.Sprintf("zarf-package-basic-pod-%s-0.0.1.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// deploy the above package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// using a pod the package deploys - run a kubectl debug command
	stdOut, stdErr, err = e2e.Kubectl(t, "debug", "test-pod", "-n", "test", "--image=ghcr.io/zarf-dev/images/alpine:3.21.3", "--profile", "general")
	require.NoError(t, err, stdOut, stdErr)

	// wait for the ephemeralContainer to exist - as it need to traverse mutation/admission
	stdOut, stdErr, err = e2e.Kubectl(t, "wait", "--namespace=test", "--for=jsonpath={.status.ephemeralContainerStatuses[*].image}=127.0.0.1:31337/zarf-dev/images/alpine:3.21.3-zarf-1792331847", "pod/test-pod", "--timeout=10s")
	require.NoError(t, err, stdOut, stdErr)

	podStdOut, stdErr, err := e2e.Kubectl(t, "get", "pod", "test-pod", "-n", "test", "-o", "jsonpath={.status.ephemeralContainerStatuses[*].image}")
	require.NoError(t, err, podStdOut, stdErr)

	// Ensure the image used contains the internal zarf registry (IE mutated)
	require.Contains(t, podStdOut, "127.0.0.1:31337/zarf-dev/images/alpine:3.21.3-zarf-1792331847")
}
