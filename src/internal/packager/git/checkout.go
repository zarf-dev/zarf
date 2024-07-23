// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// CheckoutTag performs a `git checkout` of the provided tag to a detached HEAD.
func (g *Git) CheckoutTag(tag string) error {
	message.Debugf("git checkout tag %s", tag)

	options := &git.CheckoutOptions{
		Branch: ParseRef(tag),
	}
	return g.checkout(options)
}

func (g *Git) checkoutRefAsBranch(ref string, branch plumbing.ReferenceName) error {
	if plumbing.IsHash(ref) {
		return g.checkoutHashAsBranch(plumbing.NewHash(ref), branch)
	}

	return g.checkoutTagAsBranch(ref, branch)
}

// checkoutTagAsBranch performs a `git checkout` of the provided tag but rather
// than checking out to a detached head, checks out to the provided branch ref
// It will delete the branch provided if it exists.
func (g *Git) checkoutTagAsBranch(tag string, branch plumbing.ReferenceName) error {
	message.Debugf("git checkout tag %s on %s", tag, branch)

	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	tagRef, err := repo.Tag(tag)
	if err != nil {
		return fmt.Errorf("failed to locate tag (%s) in repository: %w", tag, err)
	}

	return g.checkoutHashAsBranch(tagRef.Hash(), branch)
}

// checkoutHashAsBranch performs a `git checkout` of the commit hash associated
// with the provided hash
// It will delete the branch provided if it exists.
func (g *Git) checkoutHashAsBranch(hash plumbing.Hash, branch plumbing.ReferenceName) error {
	message.Debugf("git checkout hash %s on %s", hash, branch)

	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	objRef, err := repo.Object(plumbing.AnyObject, hash)
	if err != nil {
		return fmt.Errorf("an error occurred when getting the repo's object reference: %w", err)
	}

	var commitHash plumbing.Hash
	switch objRef := objRef.(type) {
	case *object.Tag:
		commitHash = objRef.Target
	case *object.Commit:
		commitHash = objRef.Hash
	default:
		// This shouldn't ever hit, but we should at least log it if someday it
		// does get hit
		message.Warnf("Checkout failed. Hash type %s not supported.", objRef.Type().String())
		return err
	}

	options := &git.CheckoutOptions{
		Hash:   commitHash,
		Branch: branch,
		Create: true,
		Force:  true,
	}
	return g.checkout(options)
}

// checkout performs a `git checkout` on the path provided using the options provided
// It assumes the caller knows what to do and does not perform any safety checks.
func (g *Git) checkout(checkoutOptions *git.CheckoutOptions) error {
	// Open the given repo
	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	// Get the working tree so we can change refs
	tree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("unable to load the git repo: %w", err)
	}

	// Perform the checkout
	return tree.Checkout(checkoutOptions)
}
