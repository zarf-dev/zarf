package git

import (
	"errors"
	"path"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
)

// fetchTag performs a `git fetch` of _only_ the provided tag.
func fetchTag(gitDirectory string, tag string) {
	message.Debugf("Fetch git tag %s from repo %s", tag, path.Base(gitDirectory))

	refspec := goConfig.RefSpec("refs/tags/" + tag + ":refs/tags/" + tag)

	err := fetch(gitDirectory, refspec)

	if errors.Is(err, git.ErrTagExists) || errors.Is(err, git.NoErrAlreadyUpToDate) {
		message.Debug("Tag already fetched")
	} else if err != nil {
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

// fetch performs a `git fetch` of _only_ the provided git refspec.
func fetch(gitDirectory string, refspec goConfig.RefSpec) error {
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
	message.Debugf("Attempting to find ref: %s for %s", refspec.String(), gitURL)

	gitCred := FindAuthForHost(gitURL)

	fetchOptions := &git.FetchOptions{
		RemoteName: onlineRemoteName,
		RefSpecs:   []goConfig.RefSpec{refspec},
	}

	if gitCred.Auth.Username != "" {
		fetchOptions.Auth = &gitCred.Auth
	}

	return repo.Fetch(fetchOptions)
}
