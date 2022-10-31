package git

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// removeLocalBranchRefs removes all refs that are local branches
// It returns a slice of references deleted
func (g *Git) removeLocalBranchRefs() ([]*plumbing.Reference, error) {
	return g.removeReferences(
		func(ref *plumbing.Reference) bool {
			return ref.Name().IsBranch()
		},
	)
}

// removeOnlineRemoteRefs removes all refs pointing to the online-upstream
// It returns a slice of references deleted
func (g *Git) removeOnlineRemoteRefs() ([]*plumbing.Reference, error) {
	return g.removeReferences(
		func(ref *plumbing.Reference) bool {
			return strings.HasPrefix(ref.Name().String(), onlineRemoteRefPrefix)
		},
	)
}

// removeHeadCopies removes any refs that aren't HEAD but have the same hash
// It returns a slice of references deleted
func (g *Git) removeHeadCopies() ([]*plumbing.Reference, error) {
	message.Debugf("git.removeHeadCopies()")

	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return nil, fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to identify references when getting the repo's head: %w", err)
	}

	headHash := head.Hash().String()
	return g.removeReferences(
		func(ref *plumbing.Reference) bool {
			// Don't ever remove tags
			return !ref.Name().IsTag() && ref.Hash().String() == headHash
		},
	)
}

// removeReferences removes references based on a provided callback
// removeReferences does not allow you to delete HEAD
// It returns a slice of references deleted
func (g *Git) removeReferences(shouldRemove func(*plumbing.Reference) bool) ([]*plumbing.Reference, error) {
	message.Debugf("git.removeReferences()")
	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return nil, fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	references, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to identify references when getting the repo's references: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to identify head: %w", err)
	}

	var removedRefs []*plumbing.Reference
	err = references.ForEach(func(ref *plumbing.Reference) error {
		refIsNotHeadOrHeadTarget := ref.Name() != plumbing.HEAD && ref.Name() != head.Name()
		// Run shouldRemove inline here to take advantage of short circuit
		// evaluation as to not waste a cycle on HEAD
		if refIsNotHeadOrHeadTarget && shouldRemove(ref) {
			err = repo.Storer.RemoveReference(ref.Name())
			if err != nil {
				return err
			}
			removedRefs = append(removedRefs, ref)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to remove references: %w", err)
	}

	return removedRefs, nil
}

// addRefs adds a provided arbitrary list of references to a repo
// It is intended to be used with references returned by a Remove function
func (g *Git) addRefs(refs []*plumbing.Reference) error {
	message.Debugf("git.addRefs()")
	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	for _, ref := range refs {
		err = repo.Storer.SetReference(ref)
		if err != nil {
			return fmt.Errorf("failed to add references: %w", err)
		}
	}

	return nil
}

// deleteBranchIfExists ensures the provided branch name does not exist
func (g *Git) deleteBranchIfExists(branchName plumbing.ReferenceName) error {
	message.Debugf("g.deleteBranchIfExists(%s)", branchName.String())

	repo, err := git.PlainOpen(g.GitPath)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	// Deletes the branch by name
	err = repo.DeleteBranch(branchName.Short())
	if err != nil && err != git.ErrBranchNotFound {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	// Delete reference too
	err = repo.Storer.RemoveReference(branchName)
	if err != nil && err != git.ErrInvalidReference {
		return fmt.Errorf("failed to delete branch reference: %w", err)
	}

	return nil
}
