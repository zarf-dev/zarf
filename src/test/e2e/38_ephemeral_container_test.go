// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

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
	stdOut, stdErr, err = e2e.Kubectl(t, "debug", "test-pod", "-n", "test", "--image=busybox:1.36", "--profile", "general")
	require.NoError(t, err, stdOut, stdErr)

	resultCh := make(chan string)
	defer close(resultCh)
	timeoutCh := time.After(10 * time.Second)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			podStdOut, _, err := e2e.Kubectl(t, "get", "pod", "test-pod", "-n", "test", "-o", "jsonpath={.status.ephemeralContainerStatuses[*].image}")
			if err != nil {
				// If there's an error, log it but don't fail immediately
				t.Log("Error checking ephemeral container status:", err)
				continue
			}

			if podStdOut != "" {
				t.Log("Ephemeral container detected!", podStdOut)
				resultCh <- podStdOut
				return
			}

			t.Log("Waiting for ephemeral...")
		}
	}()

	var ephemeralContainer string

	select {
	case ephemeralContainer = <-resultCh:
		t.Log("Ephemeral Container: ", ephemeralContainer)
	case <-timeoutCh:
		t.Log("Timeout reached! Ephemeral container not found.")
		t.Fail()
	}

	// Ensure the image used contains the internal zarf registry (IE mutated)
	require.Contains(t, ephemeralContainer, "127.0.0.1:31337/library/busybox:1.36-zarf-")
}
