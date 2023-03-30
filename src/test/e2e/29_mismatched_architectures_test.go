// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/stretchr/testify/require"
)

// TestMismatchedArchitectures ensures that zarf produces an error
// when the init package architecture doesn't match the target system architecture.
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

	version := "UnknownVersion"

	// This should be the name of the init package that was built during the 'Build binary and zarf packages' stage.
	initPackageName := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", e2e.arch, strings.TrimSpace(version))

	// This should be the name of the built init package with the incorrect/opposite architecture of the machine we're running on.
	mismatchedInitPackage := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", mismatchedArch, strings.TrimSpace(version))

	// Rename the init package with the mismatched architecture.
	err := os.Rename(initPackageName, mismatchedInitPackage)
	require.NoError(t, err)

	// Make sure zarf init returns an error because of the mismatched architectures.
	// We need to use the --architecture flag here to force zarf to find the renamed package.
	stdOut, stdErr, err := e2e.execZarfCommand("init", "--architecture", mismatchedArch, "--confirm")
	require.Error(t, err, stdOut, stdErr)
	require.Containsf(t, stdErr, lang.CmdInitErrVerifyArchitecture, "This error message should contain a description for mismatched architectures.")
}
