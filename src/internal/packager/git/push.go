// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories
package git

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// PushRepo pushes a git repository from the local path to the configured git server
func (g *Git) PushRepo(localPath string) error {
	spinner := message.NewProgressSpinner("Processing git repo at %s", localPath)
	defer spinner.Stop()

	g.GitPath = localPath
	basename := filepath.Base(localPath)
	spinner.Updatef("Pushing git repo %s", basename)

	repo, err := g.prepRepoForPush()
	if err != nil {
		message.Warnf("error when prepping the repo for push.. %v", err)
		return err
	}

	if err := g.push(repo, spinner); err != nil {
		spinner.Warnf("Unable to push the git repo %s", basename)
		return err
	}

	// Add the read-only user to this repo
	if g.Server.InternalServer {
		// Get the upstream URL
		remote, err := repo.Remote(onlineRemoteName)
		if err != nil {
			message.Warn("unable to get the information needed to add the read-only user to the repo")
			return err
		}
		remoteURL := remote.Config().URLs[0]
		repoName, err := g.TransformURLtoRepoName(remoteURL)
		if err != nil {
			message.Warnf("Unable to add the read-only user to the repo: %s\n", repoName)
			return err
		}

		err = g.addReadOnlyUserToRepo(g.Server.Address, repoName)
		if err != nil {
			message.Warnf("Unable to add the read-only user to the repo: %s\n", repoName)
			return err
		}
	}

	spinner.Success()
	return nil
}

func (g *Git) prepRepoForPush() (*git.Repository, error) {
	// Open the given repo
	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return nil, fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	// Get the upstream URL
	remote, err := repo.Remote(onlineRemoteName)
	if err != nil {
		return nil, fmt.Errorf("unable to find the git remote: %w", err)
	}

	remoteURL := remote.Config().URLs[0]
	targetURL, err := g.transformURL(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("unable to transform the git url: %w", err)
	}

	// Remove any preexisting offlineRemotes (happens when a retry is triggered)
	_ = repo.DeleteRemote(offlineRemoteName)

	_, err = repo.CreateRemote(&goConfig.RemoteConfig{
		Name: offlineRemoteName,
		URLs: []string{targetURL},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create offline remote: %w", err)
	}

	return repo, nil
}

func (g *Git) push(repo *git.Repository, spinner *message.Spinner) error {
	gitCred := http.BasicAuth{
		Username: g.Server.PushUsername,
		Password: g.Server.PushPassword,
	}

	// Since we are pushing HEAD:refs/heads/master on deployment, leaving
	// duplicates of the HEAD ref (ex. refs/heads/master,
	// refs/remotes/online-upstream/master, will cause the push to fail)
	removedRefs, err := g.removeHeadCopies()
	if err != nil {
		return fmt.Errorf("unable to remove unused git refs from the repo: %w", err)
	}

	// Fetch remote offline refs in case of old update or if multiple refs are specified in one package
	fetchOptions := &git.FetchOptions{
		RemoteName: offlineRemoteName,
		Auth:       &gitCred,
		RefSpecs: []goConfig.RefSpec{
			"refs/heads/*:refs/heads/*",
			onlineRemoteRefPrefix + "*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	}

	// Attempt the fetch, if it fails, log a warning and continue trying to push (might as well try..)
	err = repo.Fetch(fetchOptions)
	if errors.Is(err, transport.ErrRepositoryNotFound) {
		message.Debugf("Repo not yet available offline, skipping fetch...")
	} else if errors.Is(err, git.ErrForceNeeded) {
		message.Debugf("Repo fetch requires force, skipping fetch...")
	} else if errors.Is(err, git.NoErrAlreadyUpToDate) {
		message.Debugf("Repo already up-to-date, skipping fetch...")
	} else if err != nil {
		return fmt.Errorf("unable to fetch the git repo prior to push: %w", err)
	}

	// Push all heads and tags to the offline remote
	err = repo.Push(&git.PushOptions{
		RemoteName: offlineRemoteName,
		Auth:       &gitCred,
		Progress:   spinner,
		// If a provided refspec doesn't push anything, it is just ignored
		RefSpecs: []goConfig.RefSpec{
			"refs/heads/*:refs/heads/*",
			onlineRemoteRefPrefix + "*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	})

	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		message.Debug("Repo already up-to-date")
	} else if err != nil {
		return fmt.Errorf("unable to push repo to the gitops service: %w", err)
	}

	// Add back the refs we removed just incase this push isn't the last thing
	// being run and a later task needs to reference them.
	g.addRefs(removedRefs)

	return nil
}
