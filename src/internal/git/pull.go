package git

import (
	"context"
	"errors"
	"path/filepath"
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
	path, _ := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	// If downloading to temp, grab all tags since the repo isn't being
	// packaged anyway, and it saves us from having to fetch the tags
	// later if we need them
	pull(gitUrl, path, spinner)
	return path
}

func Pull(gitUrl, targetFolder string, spinner *message.Spinner) string {
	path := targetFolder + "/" + transformURLtoRepoName(gitUrl)
	pull(gitUrl, path, spinner)
	return path
}

func pull(gitUrl, targetFolder string, spinner *message.Spinner) {
	spinner.Updatef("Processing git repo %s", gitUrl)

	gitCred := FindAuthForHost(gitUrl)
	gitCachePath := filepath.Join(config.GetCachePath(), "repos/"+transformURLtoRepoName(gitUrl))

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

	// TODO: Refactor this iffy mess
	// Clone the given repo
	repo, err := git.PlainClone(gitCachePath, false, cloneOptions)

	if err == git.ErrRepositoryAlreadyExists {
		spinner.Debugf("Repo already cloned, fetching upstream changes...")

		fetchOptions := &git.FetchOptions{
			RemoteName: onlineRemoteName,
			Force:      true,
		}

		// Gracefully handle no git creds on the system (like our CI/CD)
		if gitCred.Auth.Username != "" {
			fetchOptions.Auth = &gitCred.Auth
		}

		repo, err = git.PlainOpen(gitCachePath)
		if err != nil {
			message.Fatal(err, "Unable to load cached git repo")
		}

		err = repo.Fetch(fetchOptions)

		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			spinner.Debugf("Repo already up to date")
		} else if err != nil {
			spinner.Debugf("Failed to fetch repo: %s", err)
			message.Infof("Falling back to host git for %s", gitUrl)

			// TODO fallback fetch workflow THIS IS ALL BAD CODE!
			// If we can't fetch with go-git, fallback to the host fetch
			// Only support "all tags" due to the azure fetch url format including a username
			stdOut, stdErr, err := utils.ExecCommandWithContext(context.TODO(), gitCachePath, false, "git", "fetch", onlineRemoteName)
			spinner.Updatef(stdOut)
			spinner.Debugf(stdErr)

			if err != nil {
				spinner.Fatalf(err, "Not a valid git repo or unable to fetch")
			}

			err = utils.CreatePathAndCopy(gitCachePath, targetFolder)
			if err != nil {
				message.Errorf(err, "Error bad %#v", err)
			}

			return
		}
	} else if err != nil {
		spinner.Debugf("Failed to clone repo: %s", err)
		message.Infof("Falling back to host git for %s", gitUrl)

		// If we can't clone with go-git, fallback to the host clone
		// Only support "all tags" due to the azure clone url format including a username
		stdOut, stdErr, err := utils.ExecCommandWithContext(context.TODO(), "", false, "git", "clone", "--origin", onlineRemoteName, gitUrl, gitCachePath)
		spinner.Updatef(stdOut)
		spinner.Debugf(stdErr)

		if err != nil {
			spinner.Fatalf(err, "Not a valid git repo or unable to clone")
		}

		err = utils.CreatePathAndCopy(gitCachePath, targetFolder)
		if err != nil {
			message.Errorf(err, "Error bad %#v", err)
		}

		return
	}

	err = utils.CreatePathAndCopy(gitCachePath, targetFolder)
	if err != nil {
		message.Errorf(err, "Error bad %#v", err)
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
