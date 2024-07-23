// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// DownloadRepoToTemp clones or updates a repo into a temp folder to perform ephemeral actions (i.e. process chart repos).
func (g *Git) DownloadRepoToTemp(ctx context.Context, gitURL string) error {
	g.Spinner.Updatef("g.DownloadRepoToTemp(%s)", gitURL)

	path, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return fmt.Errorf("unable to create tmpdir: %w", err)
	}

	// If downloading to temp, set this as a shallow clone to only pull the exact
	// gitURL w/ ref that was specified since we will throw away git history anyway
	if err = g.Pull(ctx, gitURL, path, true); err != nil {
		return fmt.Errorf("unable to pull the git repo at %s: %w", gitURL, err)
	}

	return nil
}

// Pull clones or updates a git repository into the target folder.
func (g *Git) Pull(ctx context.Context, gitURL, targetFolder string, shallow bool) error {
	g.Spinner.Updatef("Processing git repo %s", gitURL)

	// Split the remote url and the zarf reference
	gitURLNoRef, refPlain, err := transform.GitURLSplitRef(gitURL)
	if err != nil {
		return err
	}

	var ref plumbing.ReferenceName

	// Parse the ref from the git URL.
	if refPlain != emptyRef {
		ref = ParseRef(refPlain)
	}

	// Construct a path unique to this git repo
	repoFolder, err := transform.GitURLtoFolderName(gitURL)
	if err != nil {
		return err
	}

	g.GitPath = path.Join(targetFolder, repoFolder)

	// Clone the git repository.
	err = g.clone(ctx, gitURLNoRef, ref, shallow)
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
