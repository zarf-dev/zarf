package git

import (
	"path"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/go-git/go-git/v5"
	goConfig "github.com/go-git/go-git/v5/config"
)

// fetchTag performs a `git fetch` of _only_ the provided tag
func fetchTag(gitDirectory string, tag string) {
	message.Debugf("Fetch git tag %s from repo %s", tag, path.Base(gitDirectory))

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

	gitUrl := remotes[0].Config().URLs[0]
	message.Debugf("Attempting to find tag: %s for %s", tag, gitUrl)

	gitCred := FindAuthForHost(gitUrl)

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
		message.Debug("Tag already fetched")
	} else if err != nil {
		message.Fatal(err, "Not a valid tag or unable to fetch")
	}
}
