// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

func TestCreateGit(t *testing.T) {
	t.Log("E2E: Test Git Repo Behavior")

	tmpdir := t.TempDir()

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "examples/git-data", "-o", tmpdir, "--confirm")
	require.NoError(t, err, stdOut, stdErr)
	packageName := fmt.Sprintf("zarf-package-git-data-%s-0.0.1.tar.zst", e2e.Arch)
	pkgLayout, err := layout.LoadFromTar(context.Background(), filepath.Join(tmpdir, packageName), layout.PackageLayoutOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	for _, tc := range []struct {
		component, sha, head, message, tags, branches string
	}{
		{"full-repo", "0a6b587", "HEAD -> main, online-upstream/main", "Adjust dragon spacing", "v0.0.1\n", "  dragons\n* main\n"},
		{"specific-tag", "5249809", "HEAD -> zarf-ref-v0.0.1, tag: v0.0.1", "Added README.md", "v0.0.1\n", "* zarf-ref-v0.0.1\n"},
		{"specific-branch", "01a2321", "HEAD -> dragons, online-upstream/dragons", "Explain what this repo does", "", "* dragons\n"},
		{"specific-hash", "01a2321", "HEAD -> zarf-ref-01a23218923f24194133b5eb11268cf8d73ff1bb, online-upstream/dragons", "Explain what this repo does", "v0.0.1\n", "  main\n* zarf-ref-01a23218923f24194133b5eb11268cf8d73ff1bb\n"},
	} {
		t.Run(tc.component, func(t *testing.T) {
			repoDir, err := pkgLayout.GetComponentDir(t.Context(), t.TempDir(), tc.component, layout.RepoComponentDir)
			require.NoError(t, err)
			repos, err := os.ReadDir(repoDir)
			require.NoError(t, err)
			// Each ref selection is exercised through GitHub and the Azure fallback.
			require.Len(t, repos, 2)
			for _, repo := range repos {
				verifyGitRepo(t, filepath.Join(repoDir, repo.Name(), ".git"), tc.sha, tc.head, tc.message, tc.tags, tc.branches)
			}
		})
	}
}

func verifyGitRepo(t *testing.T, gitDir string, shortSha string, headTracking string, commitMsg string, tags string, branches string) {
	gitDirFlag := fmt.Sprintf("--git-dir=%s", gitDir)
	stdOut, stdErr, err := exec.Cmd("git", gitDirFlag, "log", "-n", "1", "--oneline", "--decorate=short")
	require.NoError(t, err, stdOut, stdErr)
	require.Contains(t, stdOut, shortSha)
	require.Contains(t, stdOut, headTracking)
	require.Contains(t, stdOut, commitMsg)

	// Verify the repo has its tags and branches.
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "tag")
	require.NoError(t, err, stdOut, stdErr)
	require.Equal(t, tags, stdOut)
	stdOut, stdErr, err = exec.Cmd("git", gitDirFlag, "branch")
	require.NoError(t, err, stdOut, stdErr)
	require.Equal(t, branches, stdOut)
}
