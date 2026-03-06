// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDestroy(t *testing.T) {
	t.Log("E2E: YOLO Mode")

	// Destroy the cluster to test Zarf cleaning up after itself
	stdOut, stdErr, err := e2e.Zarf(t, "destroy", "--confirm", "--remove-components")
	require.NoError(t, err, stdOut, stdErr)
}
