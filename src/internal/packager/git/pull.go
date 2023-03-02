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

	// Parse the git URL into its parts.
	get, err := g.urlParser(gitURL)
	if err != nil {
		return fmt.Errorf("unable to parse git url (%s): %w", gitURL, err)
	}

	// Get the git URL without the ref so we can clone the repo.
	gitURLNoRef := fmt.Sprintf("%s%s/%s%s", get("proto"), get("hostPath"), get("repo"), get("git"))

	// Setup git paths, including a unique name for the repo based on the hash of the git URL to avoid conflicts.
	repoName := fmt.Sprintf("%s-%d", get("repo"), utils.GetCRCHash(gitURL))
	g.GitPath = path.Join(targetFolder, repoName)

	// Setup any refs for this pull operation.
	var ref plumbing.ReferenceName
	refPlain := get("ref")
	partialClone := refPlain != ""
	if partialClone {
		ref = g.parseRef(refPlain)
	}

	// Clone the repo.
	if _, err := g.clone(g.GitPath, gitURLNoRef, ref, partialClone); err != nil {
		return fmt.Errorf("not a valid git repo or unable to clone (%s): %w", gitURL, err)
	}

	// If a non-branch ref was provided, checkout the ref as a branch so gitea doesn't have issues.
	if partialClone && !ref.IsBranch() {
		// Remove the "refs/tags/" prefix from the ref.
		stripped := strings.TrimPrefix(ref.String(), "refs/tags/")

		// Use the plain ref as part of the branch name so it is unique and doesn't conflict with other refs.
		alias := fmt.Sprintf("zarf-ref-%s", stripped)
		trunkBranchName := plumbing.NewBranchReferenceName(alias)

		// Checkout the ref as a branch.
		return g.checkoutRefAsBranch(stripped, trunkBranchName)
	}

	return nil
}

// parseRef parses the provided ref into a ReferenceName.
func (g *Git) parseRef(r string) plumbing.ReferenceName {
	// If not a full ref, assume it's a tag at this point.
	if !plumbing.IsHash(r) && !strings.HasPrefix(r, "refs/") {
		r = fmt.Sprintf("refs/tags/%s", r)
	}

	// Set the reference name to the provided ref.
	return plumbing.ReferenceName(r)
}
