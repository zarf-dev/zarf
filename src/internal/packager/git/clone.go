// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"context"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// clone performs a `git clone` of a given repo.
func (g *Git) clone(gitDirectory string, gitURL string, ref plumbing.ReferenceName) error {
	cloneOptions := &git.CloneOptions{
		URL:        gitURL,
		Progress:   g.Spinner,
		RemoteName: onlineRemoteName,
	}

	// If the ref is a tag, don't clone all tags.
	if ref.IsTag() {
		cloneOptions.Tags = git.NoTags
	}

	// Use a single branch if we're cloning a specific branch
	if ref.IsBranch() || ref.IsTag() {
		cloneOptions.ReferenceName = ref
		cloneOptions.SingleBranch = true
	}

	gitCred := utils.FindAuthForHost(gitURL)

	// Gracefully handle no git creds on the system (like our CI/CD).
	if gitCred.Auth.Username != "" {
		cloneOptions.Auth = &gitCred.Auth
	}

	// Clone the given repo.
	repo, err := git.PlainClone(gitDirectory, false, cloneOptions)
	if err != nil {
		message.Debugf("Failed to clone repo %s: %s", gitURL, err.Error())
		g.Spinner.Updatef("Falling back to host git for %s", gitURL)

		// If we can't clone with go-git, fallback to the host clone
		// Only support "all tags" due to the azure clone url format including a username
		cmdArgs := []string{"clone", "--origin", onlineRemoteName, gitURL, gitDirectory}

		// If the ref is a tag, don't clone all tags.
		if ref.IsTag() {
			cmdArgs = append(cmdArgs, "--no-tags")
		}

		// Use a single branch if we're cloning a specific branch
		if ref.IsBranch() || ref.IsTag() {
			cmdArgs = append(cmdArgs, "-b", ref.String())
			cmdArgs = append(cmdArgs, "--single-branch")
		}

		execConfig := exec.Config{
			Stdout: g.Spinner,
			Stderr: g.Spinner,
		}
		_, _, err := exec.CmdWithContext(context.TODO(), execConfig, "git", cmdArgs...)
		if err != nil {
			return err
		}

		return nil
	}

	// If we're cloning the whole repo, we need to also fetch the other branches besides the default.
	if ref == emptyRef {
		fetchOpts := &git.FetchOptions{
			RemoteName: onlineRemoteName,
			Progress:   g.Spinner,
			RefSpecs:   []goConfig.RefSpec{"refs/*:refs/*", "HEAD:refs/heads/HEAD"},
			Tags:       git.AllTags,
		}
		if err := repo.Fetch(fetchOpts); err != nil {
			return err
		}
	}

	return nil
}
