// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMismatchedArchitectures ensures that zarf produces an error
// when the init package architecture doesn't match the cluster architecture.
func TestMismatchedArchitectures(t *testing.T) {
	t.Log("E2E: Zarf init with mismatched architectures")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	var mismatchedArch string
	if e2e.arch == "amd64" {
		mismatchedArch = "arm64"
	}
	if e2e.arch == "arm64" {
		mismatchedArch = "amd64"
	}

	// // Pull the current zarf binary version to find the corresponding init package
	// version, stdErr, err := e2e.execZarfCommand("version")
	// require.NoError(t, err, version, stdErr)

	version := "UnknownVersion"

	// This should be the name of the init package that was built during the 'Build binary and zarf packages' stage.
	initPackageName := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", e2e.arch, strings.TrimSpace(version))

	// This should be the name of the built init package with the incorrect/opposite architecture of the machine we're running on.
	mismatchedInitPackage := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", mismatchedArch, strings.TrimSpace(version))

	// Rename the init package with the mismatched architecture
	err := os.Rename(initPackageName, mismatchedInitPackage)
	require.NoError(t, err)

	// Check that zarf init returned an error because of the mismatched architectures
	output, stdErr, err := e2e.execZarfCommand("init", "--confirm")
	require.Error(t, err, output, stdErr)
}
