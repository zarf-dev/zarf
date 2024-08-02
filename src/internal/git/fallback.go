// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"context"
	"io"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

// gitCloneFallback is a fallback if go-git fails to clone a repo.
func (r *Repository) gitCloneFallback(ctx context.Context, gitURL string, ref plumbing.ReferenceName, shallow bool) error {
	// If we can't clone with go-git, fallback to the host clone
	// Only support "all tags" due to the azure clone url format including a username
	cloneArgs := []string{"clone", "--origin", onlineRemoteName, gitURL, r.path}

	// Don't clone all tags / refs if we're cloning a specific tag or branch.
	if ref.IsTag() || ref.IsBranch() {
		cloneArgs = append(cloneArgs, "--no-tags")
		cloneArgs = append(cloneArgs, "-b", ref.Short())
		cloneArgs = append(cloneArgs, "--single-branch")
	}

	// If this is a shallow clone set the depth to 1
	if shallow {
		cloneArgs = append(cloneArgs, "--depth", "1")
	}

	cloneExecConfig := exec.Config{
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_, _, err := exec.CmdWithContext(ctx, cloneExecConfig, "git", cloneArgs...)
	if err != nil {
		return err
	}

	// If we're cloning the whole repo, we need to also fetch the other branches besides the default.
	if ref == emptyRef {
		fetchArgs := []string{"fetch", "--tags", "--update-head-ok", onlineRemoteName, "refs/*:refs/*"}
		fetchExecConfig := exec.Config{
			Stdout: io.Discard,
			Stderr: io.Discard,
			Dir:    r.path,
		}
		_, _, err := exec.CmdWithContext(ctx, fetchExecConfig, "git", fetchArgs...)
		if err != nil {
			return err
		}
	}

	return nil
}
