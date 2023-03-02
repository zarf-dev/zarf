// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateGit(t *testing.T) {
	extractDir := filepath.Join(os.TempDir(), ".extracted-git-pkg")

	pkgDir := "src/test/test-packages/git-repo-behavior"
	pkgPath := fmt.Sprintf("%s/zarf-package-git-behavior-%s.tar.zst", pkgDir, e2e.arch)
	outputFlag := fmt.Sprintf("-o=%s", pkgDir)
	e2e.cleanFiles(extractDir, pkgPath)

	_, _, err := e2e.execZarfCommand("package", "create", pkgDir, outputFlag, "--confirm")
	require.NoError(t, err, "error when building the test package")
	// defer e2e.cleanFiles(pkgPath)

	stdOut, stdErr, err := e2e.execZarfCommand("tools", "archiver", "decompress", pkgPath, extractDir)
	require.NoError(t, err, stdOut, stdErr)
	// defer e2e.cleanFiles(extractDir)

	// Verify the main zarf repo only has one tag
	gitDirFlag := fmt.Sprintf("--git-dir=%s/components/specific-tag/repos/zarf-1211668992/.git", extractDir)
	gitTagOut, err := exec.Command("git", gitDirFlag, "tag", "-l").Output()
	require.NoError(t, err)
	require.Equal(t, "v0.15.0\n", string(gitTagOut))

	gitHeadOut, err := exec.Command("git", gitDirFlag, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	require.Equal(t, "9eb207e552fe3a73a9ced064d35a9d9872dfbe6d\n", string(gitHeadOut))

	// Verify the second zarf repo only has two tags
	gitDirFlag = fmt.Sprintf("--git-dir=%s/components/specific-tag-update/repos/zarf-1211668992/.git", extractDir)
	gitTagOut, err = exec.Command("git", gitDirFlag, "tag", "-l").Output()
	require.NoError(t, err)
	require.Equal(t, "v0.16.0\nv0.17.0\n", string(gitTagOut))

	gitHeadOut, err = exec.Command("git", gitDirFlag, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	require.Equal(t, "bea100213565de1348375828e14be6e1482a67f8\n", string(gitHeadOut))
}
