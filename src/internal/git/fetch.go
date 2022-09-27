package git

import (
	"context"
	"errors"
	"path"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
)

// fetchTag performs a `git fetch` of _only_ the provided tag.
func fetchTag(gitDirectory string, tag string) {
	message.Debugf("Fetch git tag %s from repo %s", tag, path.Base(gitDirectory))

	refspec := goConfig.RefSpec("refs/tags/" + tag + ":refs/tags/" + tag)

	err := fetch(gitDirectory, refspec)

	if err != nil {
		message.Fatal(err, "Not a valid tag or unable to fetch")
	}
}

// fetchHash performs a `git fetch` of _only_ the provided commit hash.
func fetchHash(gitDirectory string, hash string) {
	message.Debugf("Fetch git hash %s from repo %s", hash, path.Base(gitDirectory))

	refspec := goConfig.RefSpec(hash + ":" + hash)

	err := fetch(gitDirectory, refspec)

	if err != nil {
		message.Fatal(err, "Not a valid hash or unable to fetch")
	}
}

// fetch performs a `git fetch` of _only_ the provided git refspec(s).
func fetch(gitDirectory string, refspecs ...goConfig.RefSpec) error {
	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		message.Fatal(err, "Unable to load the git repo")
	}

	remotes, err := repo.Remotes()
	// There should never be no remotes, but it's easier to account for than
	// let be a bug later
	if err != nil || len(remotes) == 0 {
		message.Fatal(err, "Failed to identify remotes.")
	}

	gitURL := remotes[0].Config().URLs[0]
	message.Debugf("Attempting to find ref: %#v for %s", refspecs, gitURL)

	gitCred := FindAuthForHost(gitURL)

	fetchOptions := &git.FetchOptions{
		RemoteName: onlineRemoteName,
		RefSpecs:   refspecs,
	}

	if gitCred.Auth.Username != "" {
		fetchOptions.Auth = &gitCred.Auth
	}

	err = repo.Fetch(fetchOptions)

	if errors.Is(err, git.ErrTagExists) || errors.Is(err, git.NoErrAlreadyUpToDate) {
		message.Debug("Already fetched requested ref")
	} else if err != nil {
		message.Debugf("Failed to fetch repo: %s", err)
		message.Infof("Falling back to host git for %s", gitURL)

		// If we can't fetch with go-git, fallback to the host fetch
		// Only support "all tags" due to the azure fetch url format including a username
		cmdArgs := []string{"fetch", onlineRemoteName}
		for _, refspec := range refspecs {
			cmdArgs = append(cmdArgs, refspec.String())
		}
		_, _, err := utils.ExecCommandWithContext(context.TODO(), gitDirectory, false, "git", cmdArgs...)

		return err
	}

	return nil
}
