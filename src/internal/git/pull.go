package git

import (
	"context"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"strings"
)

const onlineRemoteName = "online-upstream"

func DownloadRepoToTemp(gitUrl string, spinner *message.Spinner) string {
	path, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		message.Fatalf(err, "Specified tmpdir does not exist, please create it: %s", config.CommonOptions.TempDirectory)
	}
	// If downloading to temp, grab all tags since the repo isn't being
	// packaged anyway, and it saves us from having to fetch the tags
	// later if we need them
	pull(gitUrl, path, spinner)
	return path
}

func Pull(gitUrl, targetFolder string, spinner *message.Spinner) (string, error) {
	repoName, err := transformURLtoRepoName(gitUrl)
	if err != nil {
		message.Errorf(err, "unable to pull the git repo at %s", gitUrl)
		return "", err
	}

	path := targetFolder + "/" + repoName
	pull(gitUrl, path, spinner)
	return path, nil
}

func pull(gitUrl, targetFolder string, spinner *message.Spinner) {
	spinner.Updatef("Processing git repo %s", gitUrl)

	gitCred := FindAuthForHost(gitUrl)

	matches := strings.Split(gitUrl, "@")
	onlyFetchRef := len(matches) == 2
	cloneOptions := &git.CloneOptions{
		URL:        matches[0],
		Progress:   spinner,
		RemoteName: onlineRemoteName,
	}

	if onlyFetchRef {
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
		spinner.Debugf("Failed to clone repo: %s", err)
		message.Infof("Falling back to host git for %s", gitUrl)

		// If we can't clone with go-git, fallback to the host clone
		// Only support "all tags" due to the azure clone url format including a username
		stdOut, stdErr, err := utils.ExecCommandWithContext(context.TODO(), false, "git", "clone", "--origin", onlineRemoteName, gitUrl, targetFolder)
		spinner.Updatef(stdOut)
		spinner.Debugf(stdErr)

		if err != nil {
			spinner.Fatalf(err, "Not a valid git repo or unable to clone")
		}

		return
	}

	if onlyFetchRef {
		ref := matches[1]

		// Identify the remote trunk branch name
		trunkBranchName := plumbing.NewBranchReferenceName("master")
		head, err := repo.Head()

		if err != nil {
			// No repo head available
			spinner.Errorf(err, "Failed to identify repo head. Ref will be pushed to 'master'.")
		} else if head.Name().IsBranch() {
			// Valid repo head and it is a branch
			trunkBranchName = head.Name()
		} else {
			// Valid repo head but not a branch
			spinner.Errorf(nil, "No branch found for this repo head. Ref will be pushed to 'master'.")
		}

		_, _ = removeLocalBranchRefs(targetFolder)
		_, _ = removeOnlineRemoteRefs(targetFolder)

		var isHash = regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString

		if isHash(ref) {
			fetchHash(targetFolder, ref)
			checkoutHashAsBranch(targetFolder, plumbing.NewHash(ref), trunkBranchName)
		} else {
			fetchTag(targetFolder, ref)
			checkoutTagAsBranch(targetFolder, ref, trunkBranchName)
		}
	}
}
