// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"context"
	"errors"
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
)

// fetchRef performs a `git fetch` of _only_ the provided git reference (tag or hash).
func (g *Git) fetchRef(ref string) error {
	if isHash(ref) {
		return g.fetchHash(ref)
	}

	return g.fetchTag(ref)
}

// fetchTag performs a `git fetch` of _only_ the provided tag.
func (g *Git) fetchTag(tag string) error {
	refSpec := goConfig.RefSpec(fmt.Sprintf("refs/tags/%s:refs/tags/%s", tag, tag))
	fetchOptions := &git.FetchOptions{
		RemoteName: onlineRemoteName,
		RefSpecs:   []goConfig.RefSpec{refSpec},
		Tags:       git.NoTags,
	}

	return g.fetch(g.GitPath, fetchOptions)
}

// fetchHash performs a `git fetch` of _only_ the provided commit hash.
func (g *Git) fetchHash(hash string) error {
	refSpec := goConfig.RefSpec(fmt.Sprintf("%s:%s", hash, hash))
	fetchOptions := &git.FetchOptions{
		RemoteName: onlineRemoteName,
		RefSpecs:   []goConfig.RefSpec{refSpec},
		Tags:       git.NoTags,
	}

	return g.fetch(g.GitPath, fetchOptions)
}

// fetch performs a `git fetch` of _only_ the provided git refspec(s) within the fetchOptions.
func (g *Git) fetch(gitDirectory string, fetchOptions *git.FetchOptions) error {
	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		return fmt.Errorf("unable to load the git repo: %w", err)
	}

	remotes, err := repo.Remotes()
	// There should never be no remotes, but it's easier to account for than
	// let be a bug later
	if err != nil || len(remotes) == 0 {
		return fmt.Errorf("unable to identify remotes: %w", err)
	}

	gitURL := remotes[0].Config().URLs[0]
	message.Debugf("Attempting to find ref: %#v for %s", fetchOptions.RefSpecs, gitURL)

	gitCred := utils.FindAuthForHost(gitURL)

	if gitCred.Auth.Username != "" {
		fetchOptions.Auth = &gitCred.Auth
	}

	err = repo.Fetch(fetchOptions)

	if errors.Is(err, git.ErrTagExists) || errors.Is(err, git.NoErrAlreadyUpToDate) {
		message.Debug("Already fetched requested ref")
	} else if err != nil {
		message.Debugf("Failed to fetch repo %s: %s", gitURL, err.Error())
		g.Spinner.Updatef("Falling back to host git for %s", gitURL)

		// If we can't fetch with go-git, fallback to the host fetch
		// Only support "all tags" due to the azure fetch url format including a username
		cmdArgs := []string{"fetch", onlineRemoteName}
		for _, refspec := range fetchOptions.RefSpecs {
			cmdArgs = append(cmdArgs, refspec.String())
		}
		execCfg := exec.Config{
			Dir:    gitDirectory,
			Stdout: g.Spinner,
			Stderr: g.Spinner,
		}
		_, _, err := exec.CmdWithContext(context.TODO(), execCfg, "git", cmdArgs...)
		return err
	}

	return nil
}
