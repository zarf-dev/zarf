package git

import (
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"strings"
)

const onlineRemoteName = "online-upstream"

func DownloadRepoToTemp(gitUrl string, spinner *message.Spinner) string {
	path, _ := utils.MakeTempDir()
	// If downloading to temp, grab all tags since the repo isn't being
	// packaged anyway, and it saves us from having to fetch the tags
	// later if we need them
	pull(gitUrl, path, spinner)
	return path
}

func Pull(gitUrl string, targetFolder string, spinner *message.Spinner) string {
	path := targetFolder + "/" + transformURLtoRepoName(gitUrl)
	pull(gitUrl, path, spinner)
	return path
}

func pull(gitUrl string, targetFolder string, spinner *message.Spinner) {
	spinner.Updatef("Processing git repo %s", gitUrl)

	gitCred := FindAuthForHost(gitUrl)

	matches := strings.Split(gitUrl, "@")
	fetchAllTags := len(matches) == 1
	cloneOptions := &git.CloneOptions{
		URL:        matches[0],
		Progress:   spinner,
		RemoteName: onlineRemoteName,
	}

	if !fetchAllTags {
		cloneOptions.Tags = git.NoTags
	}

	// Gracefully handle no git creds on the system (like our CI/CD)
	if gitCred.Auth.Username != "" {
		cloneOptions.Auth = &gitCred.Auth
	}

	// Clone the given repo
	repo, err := git.PlainClone(targetFolder, false, cloneOptions)

	if err == git.ErrRepositoryAlreadyExists {
		spinner.Debugf("Repo already cloned")
	} else if err != nil {
		spinner.Fatalf(err, "Not a valid git repo or unable to clone")
	}

	if !fetchAllTags {
		tag := matches[1]

		// Identify the remote trunk branch name
		trunkBranchName := plumbing.NewBranchReferenceName("master")
		head, err := repo.Head()

		if err != nil {
			// No repo head available
			spinner.Errorf(err, "Failed to identify repo head. Tag will be pushed to 'master'.")
		} else if head.Name().IsBranch() {
			// Valid repo head and it is a branch
			trunkBranchName = head.Name()
		} else {
			// Valid repo head but not a branch
			spinner.Errorf(nil, "No branch found for this repo head. Tag will be pushed to 'master'.")
		}

		_, _ = removeLocalBranchRefs(targetFolder)
		_, _ = removeOnlineRemoteRefs(targetFolder)

		fetchTag(targetFolder, tag)
		CheckoutTagAsBranch(targetFolder, tag, trunkBranchName)

	}
}
