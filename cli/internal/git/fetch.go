package git

import (
	"path"

	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
	"github.com/sirupsen/logrus"
)

// FetchTag performs a `git fetch` of _only_ the provided tag
func FetchTag(gitDirectory string, tag string) {
	logContext := logrus.WithFields(logrus.Fields{
		// Base should be similar to the repo name
		"Repo": path.Base(gitDirectory),
	})

	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		logContext.Fatal(err)
	}

	remotes, err := repo.Remotes()
	// There should never be no remotes, but it's easier to account for than
	// let be a bug later
	if err != nil || len(remotes) == 0 {
		if err != nil {
			logContext.Debug(err)
		}
		logContext.Fatal("Failed to identify remotes.")
	}

	gitUrl := remotes[0].Config().URLs[0]
	// Now that we have an exact match, we may as well update the logger,
	// especially since nothing has been logged to this point that hasn't been
	// fatal.
	logContext = logrus.WithFields(logrus.Fields{
		"Remote": gitUrl,
	})

	gitCred := FindAuthForHost(gitUrl)

	logContext.Debug("Attempting to find tag: " + tag)
	fetchOptions := &git.FetchOptions{
		RemoteName: onlineRemoteName,
		RefSpecs: []goConfig.RefSpec{
			goConfig.RefSpec("refs/tags/" + tag + ":refs/tags/" + tag),
		},
	}

	if gitCred.Auth.Username != "" {
		fetchOptions.Auth = &gitCred.Auth
	}

	err = repo.Fetch(fetchOptions)

	if err == git.ErrTagExists {
		logContext.Info("Tag already fetched")
	} else if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Not a valid tag or unable to fetch")
	}

	logContext.Info("Git tag fetched")
}
