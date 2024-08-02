// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

func TestCreateGit(t *testing.T) {
	t.Log("E2E: Test Git Repo Behavior")

	tmpdir := t.TempDir()
	extractDir := filepath.Join(tmpdir, ".extracted-git-pkg")

	// Extract the test package.
	path := fmt.Sprintf("build/zarf-package-git-data-%s-0.0.1.tar.zst", e2e.Arch)
	stdOut, stdErr, err := e2e.Zarf(t, "tools", "archiver", "decompress", path, extractDir, "--unarchive-all")
	require.NoError(t, err, stdOut, stdErr)
	defer e2e.CleanFiles(extractDir)

	// Verify the full-repo component
	gitDir := fmt.Sprintf("%s/components/full-repo/repos/zarf-public-test-2395699829/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"0a6b587", "(HEAD -> main, online-upstream/main)", "Adjust dragon spacing",
		"v0.0.1\n", "  dragons\n* main\n")

	// Verify the full-repo component fallback
	gitDir = fmt.Sprintf("%s/components/full-repo/repos/zarf-public-test-410141584/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"0a6b587", "(HEAD -> main, online-upstream/main, online-upstream/HEAD)", "Adjust dragon spacing",
		"v0.0.1\n", "  dragons\n* main\n")

	// Verify specific tag component shorthand tag
	gitDir = fmt.Sprintf("%s/components/specific-tag/repos/zarf-public-test-470731282/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"5249809", "(HEAD -> zarf-ref-v0.0.1, tag: v0.0.1)", "Added README.md",
		"v0.0.1\n", "* zarf-ref-v0.0.1\n")

	// Verify specific tag component refspec tag
	gitDir = fmt.Sprintf("%s/components/specific-tag/repos/zarf-public-test-482869567/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"5249809", "(HEAD -> zarf-ref-v0.0.1, tag: v0.0.1)", "Added README.md",
		"v0.0.1\n", "* zarf-ref-v0.0.1\n")

	// Verify specific tag component tag fallback
	gitDir = fmt.Sprintf("%s/components/specific-tag/repos/zarf-public-test-3956869879/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"5249809", "(HEAD -> zarf-ref-v0.0.1, tag: v0.0.1)", "Added README.md",
		"v0.0.1\n", "* zarf-ref-v0.0.1\n")

	// Verify specific branch component
	gitDir = fmt.Sprintf("%s/components/specific-branch/repos/zarf-public-test-2265377406/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"01a2321", "(HEAD -> dragons, online-upstream/dragons)", "Explain what this repo does",
		"", "* dragons\n")

	// Verify specific branch component fallback
	gitDir = fmt.Sprintf("%s/components/specific-branch/repos/zarf-public-test-3363080017/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"01a2321", "(HEAD -> dragons, online-upstream/dragons)", "Explain what this repo does",
		"", "* dragons\n")

	// Verify specific hash component
	gitDir = fmt.Sprintf("%s/components/specific-hash/repos/zarf-public-test-3231174532/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"01a2321", "(HEAD -> zarf-ref-01a23218923f24194133b5eb11268cf8d73ff1bb, online-upstream/dragons)", "Explain what this repo does",
		"v0.0.1\n", "  main\n* zarf-ref-01a23218923f24194133b5eb11268cf8d73ff1bb\n")

	// Verify specific hash component fallback
	gitDir = fmt.Sprintf("%s/components/specific-hash/repos/zarf-public-test-1425142831/.git", extractDir)
	verifyGitRepo(t, gitDir,
		"01a2321", "(HEAD -> zarf-ref-01a23218923f24194133b5eb11268cf8d73ff1bb, online-upstream/dragons)", "Explain what this repo does",
		"v0.0.1\n", "  main\n* zarf-ref-01a23218923f24194133b5eb11268cf8d73ff1bb\n")
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
