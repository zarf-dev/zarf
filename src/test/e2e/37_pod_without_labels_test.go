// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPodWithoutLabels(t *testing.T) {
	t.Log("E2E: Pod Without Labels")
	e2e.SetupWithCluster(t)

	// Path to pod manifest containing 0 lavbels
	buildPath := filepath.Join("src", "test", "packages", "37-pod-without-labels", "pod.yaml")

	// Create the testing namespace
	_, _, err := e2e.Kubectl("create", "ns", "pod-label")
	require.NoError(t, err)

	// Create the pod without labels
	// This is not an image zarf will have in the registry - but the agent was failing to admit on an internal server error before completing admission
	_, _, err = e2e.Kubectl("create", "-f", buildPath, "-n", "pod-label")
	require.NoError(t, err)

	// Cleanup
	_, _, err = e2e.Kubectl("delete", "-f", buildPath, "-n", "pod-label")
	require.NoError(t, err)
	_, _, err = e2e.Kubectl("delete", "ns", "pod-label")
	require.NoError(t, err)
}
