package git

import (
	"context"
	"errors"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/go-git/go-git/v5"
)

// clone performs a `git clone` of a given repo.
func clone(gitDirectory string, gitURL string, onlyFetchRef bool, spinner *message.Spinner) (*git.Repository, error) {
	cloneOptions := &git.CloneOptions{
		URL:        gitURL,
		Progress:   spinner,
		RemoteName: onlineRemoteName,
	}

	if onlyFetchRef {
		cloneOptions.Tags = git.NoTags
	}

	gitCred := FindAuthForHost(gitURL)

	// Gracefully handle no git creds on the system (like our CI/CD)
	if gitCred.Auth.Username != "" {
		cloneOptions.Auth = &gitCred.Auth
	}

	// Clone the given repo
	repo, err := git.PlainClone(gitDirectory, false, cloneOptions)

	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		repo, err = git.PlainOpen(gitDirectory)

		if err != nil {
			return nil, err
		}

		return repo, git.ErrRepositoryAlreadyExists
	} else if err != nil {
		spinner.Debugf("Failed to clone repo: %s", err)
		message.Infof("Falling back to host git for %s", gitURL)

		// If we can't clone with go-git, fallback to the host clone
		// Only support "all tags" due to the azure clone url format including a username
		cmdArgs := []string{"clone", "--origin", onlineRemoteName, gitURL, gitDirectory}

		if onlyFetchRef {
			cmdArgs = append(cmdArgs, "--no-tags")
		}

		stdOut, stdErr, err := utils.ExecCommandWithContext(context.TODO(), "", false, "git", cmdArgs...)
		spinner.Updatef(stdOut)
		spinner.Debugf(stdErr)

		if err != nil {
			return nil, err
		}

		return git.PlainOpen(gitDirectory)
	} else {
		return repo, nil
	}
}
