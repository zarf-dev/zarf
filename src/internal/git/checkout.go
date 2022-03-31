package git

import (
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CheckoutTag performs a `git checkout` of the provided tag to a detached HEAD
func CheckoutTag(path string, tag string) {
	options := &git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/tags/" + tag),
	}
	checkout(path, options)
}

// CheckoutTagAsBranch performs a `git checkout` of the provided tag but rather
// than checking out to a detached head, checks out to the provided branch ref
// It will delete the branch provided if it exists
func CheckoutTagAsBranch(path string, tag string, branch plumbing.ReferenceName) {
	message.Debugf("Checkout tag %s as branch %s for %s", tag, branch.String(), path)
	repo, err := git.PlainOpen(path)
	if err != nil {
		message.Fatal(err, "Not a valid git repo or unable to open")
	}

	tagRef, err := repo.Tag(tag)
	if err != nil {
		message.Fatal(err, "Failed to locate tag in repository.")
	}
	checkoutHashAsBranch(path, tagRef.Hash(), branch)
}

// checkoutHashAsBranch performs a `git checkout` of the commit hash associated
// with the provided hash
// It will delete the branch provided if it exists
func checkoutHashAsBranch(path string, hash plumbing.Hash, branch plumbing.ReferenceName) {
	message.Debugf("Checkout hash %s as branch %s for %s", hash.String(), branch.String(), path)

	_ = deleteBranchIfExists(path, branch)

	repo, err := git.PlainOpen(path)
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
	checkout(path, options)
}

// checkout performs a `git checkout` on the path provided using the options provided
// It assumes the caller knows what to do and does not perform any safety checks
func checkout(path string, checkoutOptions *git.CheckoutOptions) {
	message.Debugf("Git checkout %s", path)

	// Open the given repo
	repo, err := git.PlainOpen(path)
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
