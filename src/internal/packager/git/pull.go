// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"fmt"
	"path"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
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
	// later if we need them
	if err = g.Pull(gitURL, path); err != nil {
		return "", fmt.Errorf("unable to pull the git repo at %s: %w", gitURL, err)
	}

	return path, nil
}

// Pull clones or updates a git repository into the target folder.
func (g *Git) Pull(gitURL, targetFolder string) error {
	g.Spinner.Updatef("Processing git repo %s", gitURL)

	// Find the repo name from the git URL.
	repoName, err := g.TransformURLtoRepoName(gitURL)
	if err != nil {
		message.Errorf(err, "unable to pull the git repo at %s", gitURL)
		return err
	}

	// Setup git paths.
	g.GitPath = path.Join(targetFolder, repoName)

	// Parse the git URL into its components.
	matches := gitURLRegex.FindStringSubmatch(gitURL)
	get := func(name string) string {
		return matches[gitURLRegex.SubexpIndex(name)]
	}

	// If unable to find a substring match for the regex, return an error.
	if len(matches) == 0 {
		return fmt.Errorf("unable to get extract the repoName from the url %s", gitURL)
	}

	refPlain := get("ref")
	partialClone := refPlain != ""

	var ref plumbing.ReferenceName

	// Parse the ref from the git URL.
	if partialClone {
		ref = g.parseRef(refPlain)
	}

	// Parse the git URL into its components.
	gitURLNoRef := fmt.Sprintf("%s%s/%s%s", get("proto"), get("hostPath"), get("repo"), get("git"))

	// Clone the repo into the cache.
	if _, err := g.clone(g.GitPath, gitURLNoRef, ref); err != nil {
		return fmt.Errorf("not a valid git repo or unable to clone (%s): %w", gitURL, err)
	}

	if partialClone && !ref.IsBranch() {
		// Remove the "refs/tags/" prefix from the ref.
		stripped := strings.TrimPrefix(ref.String(), "refs/tags/")

		// Use the stripped ref as the branch name.
		alias := fmt.Sprintf("zarf-ref-%s", stripped)
		trunkBranchName := plumbing.NewBranchReferenceName(alias)

		// Checkout the ref as a branch.
		return g.checkoutRefAsBranch(stripped, trunkBranchName)
	}

	return nil
}

// parseRef parses the provided ref into a ReferenceName if it's not a hash.
func (g *Git) parseRef(r string) plumbing.ReferenceName {
	// If not a full ref, assume it's a tag at this point.
	if !plumbing.IsHash(r) && !strings.HasPrefix(r, "refs/") {
		r = fmt.Sprintf("refs/tags/%s", r)
	}

	// Set the reference name to the provided ref.
	return plumbing.ReferenceName(r)
}
