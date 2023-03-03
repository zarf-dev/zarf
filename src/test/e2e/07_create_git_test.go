// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/stretchr/testify/require"
)

func TestCreateGit(t *testing.T) {
	t.Log("E2E: Test Git Repo Behavior")

	extractDir := filepath.Join(os.TempDir(), ".extracted-git-pkg")
	e2e.cleanFiles(extractDir)

	// Extract the test package.
	path := fmt.Sprintf("build/zarf-package-git-data-%s-v1.0.0.tar.zst", e2e.arch)
	stdOut, stdErr, err := e2e.execZarfCommand("tools", "archiver", "decompress", path, extractDir, "--decompress-all")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.cleanFiles(extractDir)

	// Verify the full-repo component.
	gitDirFlag := fmt.Sprintf("--git-dir=%s/components/full-repo/repos/nocode-953829860/.git", extractDir)
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "log", "--oneline", "--decorate")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "c46f06e add no code")
	require.Contains(t, stdOut, "(tag: 1.0.0)")
	require.Contains(t, stdOut, "(HEAD -> master, online-upstream/master, HEAD)")

	// Verify a repo with a shorthand tag.
	gitDirFlag = fmt.Sprintf("--git-dir=%s/components/specific-tag/repos/zarf-4023393304/.git", extractDir)
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "log", "HEAD^..HEAD", "--oneline", "--decorate")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "9eb207e (HEAD -> zarf-ref-v0.15.0, tag: v0.15.0) Normalize --confirm behavior in the CLI (#297)")

	// Verify a repo with a shorthand tag only has one tag.
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "tag")
	require.NoError(t, err, stdOut, stdErr)
	require.Equal(t, "v0.15.0\n", stdOut)

	// Verify a repo with a full git refspec tag.
	gitDirFlag = fmt.Sprintf("--git-dir=%s/components/specific-tag/repos/zarf-2175050463/.git", extractDir)
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "log", "HEAD^..HEAD", "--oneline", "--decorate")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "58e3cd5 (HEAD -> zarf-ref-v0.16.0, tag: v0.16.0) slightly re-arrange zarf arch diagram layout (#383)")

	// Verify a repo with a full git refspec tag only has one tag.
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "tag")
	require.NoError(t, err, stdOut, stdErr)
	require.Equal(t, "v0.16.0\n", stdOut)

	// Verify a repo with a branch.
	gitDirFlag = fmt.Sprintf("--git-dir=%s/components/specific-branch/repos/bigbang-3067531188/.git", extractDir)
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "log", "HEAD^..HEAD", "--oneline", "--decorate")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "ab6407fc (HEAD -> release-1.53.x, tag: 1.53.0-rc.1, tag: 1.53.0, online-upstream/release-1.53.x)")

	// Verify a repo with a branch only has one branch.
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "branch")
	require.NoError(t, err, stdOut, stdErr)
	require.Equal(t, "* release-1.53.x\n", stdOut)

	// Verify a repo with a commit hash.
	gitDirFlag = fmt.Sprintf("--git-dir=%s/components/specific-hash/repos/zarf-1356873667/.git", extractDir)
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "log", "HEAD^..HEAD", "--oneline", "--decorate")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, "c74e2e9 (HEAD -> zarf-ref-c74e2e9626da0400e0a41e78319b3054c53a5d4e, tag: v0.21.3) Re-add docker buildx for release pipeilne")
}
