// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"fmt"
	"path"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/go-git/go-git/v5/plumbing"
)

// DownloadRepoToTemp clones or updates a repo into a temp folder to perform ephemeral actions (i.e. process chart repos).
func (g *Git) DownloadRepoToTemp(gitURL string) (path string, err error) {
	if path, err = utils.MakeTempDir(config.CommonOptions.TempDirectory); err != nil {
		return "", fmt.Errorf("unable to create tmpdir: %w", err)
	}

	// If downloading to temp, grab all tags since the repo isn't being
	// packaged anyway, and it saves us from having to fetch the tags
	// later if we need them.
	if err = g.Pull(gitURL, path); err != nil {
		return "", fmt.Errorf("unable to pull the git repo at %s: %w", gitURL, err)
	}

	return path, nil
}

// Pull clones or updates a git repository into the target folder.
func (g *Git) Pull(gitURL, targetFolder string) error {
	g.Spinner.Updatef("Processing git repo %s", gitURL)

	// Find the Zarf-specific repo name from the git URL.
	get, err := utils.MatchRegex(gitURLRegex, gitURL)
	if err != nil {
		return fmt.Errorf("unable to parse git url (%s): %w", gitURL, err)
	}

	// Setup the reference for this repository
	refPlain := get("ref")

	var ref plumbing.ReferenceName

	// Parse the ref from the git URL.
	if refPlain != emptyRef {
		ref = g.parseRef(refPlain)
	}

	// Construct a path unique to this git repo
	repoFolder := fmt.Sprintf("%s-%d", get("repo"), utils.GetCRCHash(gitURL))
	g.GitPath = path.Join(targetFolder, repoFolder)

	// Construct the remote URL without the reference
	gitURLNoRef := fmt.Sprintf("%s%s/%s%s", get("proto"), get("hostPath"), get("repo"), get("git"))

	// Clone the git repository.
	err = g.clone(gitURLNoRef, ref)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to clone (%s): %w", gitURL, err)
	}

	if ref != emptyRef && !ref.IsBranch() {
		// Remove the "refs/tags/" prefix from the ref.
		stripped := strings.TrimPrefix(refPlain, "refs/tags/")

		// Use the plain ref as part of the branch name so it is unique and doesn't conflict with other refs.
		alias := fmt.Sprintf("zarf-ref-%s", stripped)
		trunkBranchName := plumbing.NewBranchReferenceName(alias)

		// Checkout the ref as a branch.
		return g.checkoutRefAsBranch(stripped, trunkBranchName)
	}

	return nil
}
