package git

import (
	"os"

	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/sirupsen/logrus"

	"strings"
)

const onlineRemoteName = "online-upstream"

func DownloadRepoToTemp(gitUrl string) string {
	path := utils.MakeTempDir()
	// If downloading to temp, grab all tags since the repo isn't being
	// packaged anyways and it saves us from having to fetch the tags
	// later if we need them
	pull(gitUrl, path)
	return path
}

func Pull(gitUrl string, targetFolder string) string {
	path := targetFolder + "/" + transformURLtoRepoName(gitUrl)
	pull(gitUrl, path)
	return path
}

func pull(gitUrl string, targetFolder string) {
	logContext := logrus.WithFields(logrus.Fields{
		"Remote": gitUrl,
	})
	logContext.Info("Processing git repo")

	gitCred := FindAuthForHost(gitUrl)

	matches := strings.Split(gitUrl, "@")
	fetchAllTags := len(matches) == 1
	cloneOptions := &git.CloneOptions{
		URL:        matches[0],
		Progress:   os.Stdout,
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
		logContext.Info("Repo already cloned")
	} else if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Not a valid git repo or unable to clone")
	}

	if !fetchAllTags {
		tag := matches[1]

		// Identify the remote trunk branch name
		trunkBranchName := plumbing.NewBranchReferenceName("master")
		head, err := repo.Head()

		if err != nil {
			// No repo head available
			logContext.Debug(err)
			logContext.Warn("Failed to identify repo head. Tag will be pushed to 'master'.")
		} else if head.Name().IsBranch() {
			// Valid repo head and it is a branch
			trunkBranchName = head.Name()
		} else {
			// Valid repo head but not a branch
			logContext.Warn("No branch found for this repo head. Tag will be pushed to 'master'.")
		}

		RemoveLocalBranchRefs(targetFolder)
		RemoveOnlineRemoteRefs(targetFolder)

		FetchTag(targetFolder, tag)
		CheckoutTagAsBranch(targetFolder, tag, trunkBranchName)
	}

	logContext.Info("Git repo synced")
}
