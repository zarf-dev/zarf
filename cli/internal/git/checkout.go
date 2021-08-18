package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sirupsen/logrus"
)

func CheckoutTag(path string, tag string) {

	logContext := logrus.WithFields(logrus.Fields{
		"Path": path,
		"Tag":  tag,
	})

	// Open the given repo
	repo, err := git.PlainOpen(path)
	if err != nil {
		logContext.Fatal("Not a valid git repo or unable to open")
		return
	}

	// Get the working tree so we can change refs
	tree, err := repo.Worktree()
	if err != nil {
		logContext.Fatal("Unable to load the git repo")
	}

	// Checkout our tag
	err = tree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/tags/" + tag),
	})
	if err != nil {
		logContext.Fatal("Unable to checkout the given tag")
	}
}
