// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Checkout the git repository for the provided refrence. In order of precidence
// to match the preceidence order of flux GitRepository objects
// 1. commit hash
// 2. tag
// 3. branch
func (g *Git) Checkout(refrence string) error {
	message.Debugf("git.Checkout(%s)", refrence)
	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return err
	}
	// Get the working tree so we can change refs
	tree, err := repo.Worktree()
	if err != nil {
		message.Fatal(err, "Unable to load the git repo")
	}

	// Commit Hash
	options := &git.CheckoutOptions{
		Hash: plumbing.NewHash(refrence),
		// Branch: plumbing.ReferenceName("refs/tags/" + tag),
	}

	// Perform the checkout
	err = tree.Checkout(options)
	if err == nil {
		return nil
	}
	// not a git commit
	message.Debugf("git.Checkout(%s): not a git commit: %v", refrence, err)

	options.Hash = plumbing.ZeroHash
	options.Branch = plumbing.ReferenceName("refs/tags/" + refrence)
	// Perform the checkout
	err = tree.Checkout(options)
	if err == nil {
		return nil
	}
	// not a git commit
	message.Debugf("git.Checkout(%s): not a tag.: %v", refrence, err)

	options.Branch = plumbing.ReferenceName(fmt.Sprintf("refs/remotes/%s/%s", onlineRemoteName, refrence))
	// Perform the checkout
	err = tree.Checkout(options)
	if err == nil {
		return nil
	}
	// not a git commit
	message.Debugf("git.Checkout(%s): not a branch.: %v", refrence, err)
	return err

}

// CheckoutTag performs a `git checkout` of the provided tag to a detached HEAD.
func (g *Git) CheckoutTag(tag string) {
	message.Debugf("git.CheckoutTag(%s)", tag)

	options := &git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/tags/" + tag),
	}
	g.checkout(options)
}

// checkoutTagAsBranch performs a `git checkout` of the provided tag but rather
// than checking out to a detached head, checks out to the provided branch ref
// It will delete the branch provided if it exists.
func (g *Git) checkoutTagAsBranch(tag string, branch plumbing.ReferenceName) {
	message.Debugf("git.checkoutTagAsBranch(%s,%s)", tag, branch.String())

	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		message.Fatal(err, "Not a valid git repo or unable to open")
	}

	tagRef, err := repo.Tag(tag)
	if err != nil {
		message.Fatal(err, "Failed to locate tag in repository.")
	}
	g.checkoutHashAsBranch(tagRef.Hash(), branch)
}

// checkoutHashAsBranch performs a `git checkout` of the commit hash associated
// with the provided hash
// It will delete the branch provided if it exists.
func (g *Git) checkoutHashAsBranch(hash plumbing.Hash, branch plumbing.ReferenceName) {
	message.Debugf("git.checkoutHasAsBranch(%s,%s)", hash.String(), branch.String())

	_ = g.deleteBranchIfExists(branch)

	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		message.Fatal(err, "Not a valid git repo or unable to open")
	}

	objRef, err := repo.Object(plumbing.AnyObject, hash)
	if err != nil {
		message.Fatal(err, "An error occurred when getting the repo's object reference")
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
		message.Fatalf(err, "Checkout failed. Hash type %s not supported.", objRef.Type().String())
	}

	options := &git.CheckoutOptions{
		Hash:   commitHash,
		Branch: branch,
		Create: true,
	}
	g.checkout(options)
}

// checkout performs a `git checkout` on the path provided using the options provided
// It assumes the caller knows what to do and does not perform any safety checks.
func (g *Git) checkout(checkoutOptions *git.CheckoutOptions) {
	message.Debugf("git.checkout(%#v)", checkoutOptions)

	// Open the given repo
	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		message.Fatal(err, "Not a valid git repo or unable to open")
	}

	// Get the working tree so we can change refs
	tree, err := repo.Worktree()
	if err != nil {
		message.Fatal(err, "Unable to load the git repo")
	}

	// Perform the checkout
	err = tree.Checkout(checkoutOptions)
	if err != nil {
		message.Fatal(err, "Unable to perform checkout")
	}
}
