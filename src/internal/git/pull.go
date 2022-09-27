package git

import (
	"errors"
	"fmt"
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

// DownloadRepoToTemp clones or updates a repo into a temp folder to perform ephemeral actions (i.e. process chart repos).
func DownloadRepoToTemp(gitURL string, spinner *message.Spinner) string {
	path, _ := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	// If downloading to temp, grab all tags since the repo isn't being
	// packaged anyway, and it saves us from having to fetch the tags
	// later if we need them
	pull(gitURL, path, spinner, "")
	return path
}

// Pull clones or updates a git repository into the target folder.
func Pull(gitURL, targetFolder string, spinner *message.Spinner) (string, error) {
	repoName, err := transformURLtoRepoName(gitURL)
	if err != nil {
		message.Errorf(err, "unable to pull the git repo at %s", gitURL)
		return "", err
	}

	path := targetFolder + "/" + repoName
	pull(gitURL, path, spinner, repoName)
	return path, nil
}

func pull(gitURL, targetFolder string, spinner *message.Spinner, repoName string) {
	spinner.Updatef("Processing git repo %s", gitURL)

	gitCachePath := targetFolder
	if repoName != "" {
		gitCachePath = filepath.Join(config.GetCachePath(), fmt.Sprintf("repos/%s", repoName))
	}

	matches := strings.Split(gitURL, "@")
	onlyFetchRef := len(matches) == 2

	repo, err := clone(gitCachePath, matches[0], onlyFetchRef, spinner)

	if err == git.ErrRepositoryAlreadyExists {
		spinner.Debugf("Repo already cloned, fetching upstream changes...")

		err = fetch(gitCachePath)

		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			spinner.Debugf("Repo already up to date")
		} else if err != nil {
			spinner.Fatalf(err, "Not a valid git repo or unable to fetch")
		}
	} else if err != nil {
		spinner.Fatalf(err, "Not a valid git repo or unable to clone")
	}

	if gitCachePath != targetFolder {
		err = utils.CreatePathAndCopy(gitCachePath, targetFolder)
		if err != nil {
			message.Errorf(err, "Unable to copy %s into %s", gitCachePath, targetFolder)
		}
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
