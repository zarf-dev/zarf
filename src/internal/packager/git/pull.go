package git

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// DownloadRepoToTemp clones or updates a repo into a temp folder to perform ephemeral actions (i.e. process chart repos).
func (g *Git) DownloadRepoToTemp(gitURL string) string {
	path, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		message.Fatalf(err, "Unable to create tmpdir: %s", config.CommonOptions.TempDirectory)
	}
	// If downloading to temp, grab all tags since the repo isn't being
	// packaged anyway, and it saves us from having to fetch the tags
	// later if we need them
	g.pull(gitURL, path, "")
	return path
}

// Pull clones or updates a git repository into the target folder.
func (g *Git) Pull(gitURL, targetFolder string) (path string, err error) {
	repoName, err := g.transformURLtoRepoName(gitURL)
	if err != nil {
		message.Errorf(err, "unable to pull the git repo at %s", gitURL)
		return "", err
	}

	path = targetFolder + "/" + repoName
	g.pull(gitURL, path, repoName)
	return path, nil
}

func (g *Git) pull(gitURL, targetFolder string, repoName string) {
	g.Spinner.Updatef("Processing git repo %s", gitURL)

	gitCachePath := targetFolder
	if repoName != "" {
		gitCachePath = filepath.Join(config.GetAbsCachePath(), filepath.Join(config.ZarfGitCacheDir, repoName))
	}

	matches := gitURLRegex.FindStringSubmatch(gitURL)
	idx := gitURLRegex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		message.Fatalf("unable to get extract the repoName from the url %s", gitURL)
	}

	onlyFetchRef := matches[idx("atRef")] != ""
	gitURLNoRef := fmt.Sprintf("%s%s/%s%s", matches[idx("proto")], matches[idx("hostPath")], matches[idx("repo")], matches[idx("git")])

	repo, err := g.clone(gitCachePath, gitURLNoRef, onlyFetchRef)

	if err == git.ErrRepositoryAlreadyExists {
		g.Spinner.Debugf("Repo already cloned, fetching upstream changes...")

		g.GitPath = gitCachePath
		err = g.fetch()

		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			g.Spinner.Debugf("Repo already up to date")
		} else if err != nil {
			g.Spinner.Fatalf(err, "Not a valid git repo or unable to fetch")
		}
	} else if err != nil {
		g.Spinner.Fatalf(err, "Not a valid git repo or unable to clone")
	}

	if gitCachePath != targetFolder {
		err = utils.CreatePathAndCopy(gitCachePath, targetFolder)
		if err != nil {
			message.Fatalf(err, "Unable to copy %s into %s: %#v", gitCachePath, targetFolder, err.Error())
		}
	}

	if onlyFetchRef {
		ref := matches[idx("ref")]

		// Identify the remote trunk branch name
		trunkBranchName := plumbing.NewBranchReferenceName("master")
		head, err := repo.Head()

		if err != nil {
			// No repo head available
			g.Spinner.Errorf(err, "Failed to identify repo head. Ref will be pushed to 'master'.")
		} else if head.Name().IsBranch() {
			// Valid repo head and it is a branch
			trunkBranchName = head.Name()
		} else {
			// Valid repo head but not a branch
			g.Spinner.Errorf(nil, "No branch found for this repo head. Ref will be pushed to 'master'.")
		}

		_, _ = g.removeLocalBranchRefs()
		_, _ = g.removeOnlineRemoteRefs()

		var isHash = regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString

		if isHash(ref) {
			g.fetchHash(ref)
			g.checkoutHashAsBranch(plumbing.NewHash(ref), trunkBranchName)
		} else {
			g.fetchTag(ref)
			g.checkoutTagAsBranch(ref, trunkBranchName)
		}
	}
}
