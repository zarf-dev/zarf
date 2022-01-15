package git

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type Credential struct {
	Path string
	Auth http.BasicAuth
}

func MutateGitUrlsInText(host string, text string) string {
	extractPathRegex := regexp.MustCompilePOSIX(`https?://[^/]+/(.*\.git)`)
	output := extractPathRegex.ReplaceAllStringFunc(text, func(match string) string {
		if strings.Contains(match, "/zarf-git-user/") {
			message.Warnf("%s seems to have been previously patched.", match)
			return match
		}
		return transformURL(host, match)
	})
	return output
}

func transformURLtoRepoName(url string) string {
	replaceRegex := regexp.MustCompile(`(https?://|[^\w\-.])+`)
	return "mirror" + replaceRegex.ReplaceAllString(url, "__")
}

func transformURL(baseUrl string, url string) string {
	replaced := transformURLtoRepoName(url)
	output := baseUrl + "/zarf-git-user/" + replaced
	message.Debugf("Rewrite git URL: %s -> %s", url, output)
	return output
}

func credentialFilePath() string {
	homePath, _ := os.UserHomeDir()
	return homePath + "/.git-credentials"
}

func credentialParser() []Credential {
	credentialsPath := credentialFilePath()
	var credentials []Credential

	credentialsFile, _ := os.Open(credentialsPath)
	defer func(credentialsFile *os.File) {
		err := credentialsFile.Close()
		if err != nil {
			message.Error(err, "Unable to load an existing git credentials file")
		}
	}(credentialsFile)

	scanner := bufio.NewScanner(credentialsFile)
	for scanner.Scan() {
		gitUrl, err := url.Parse(scanner.Text())
		password, _ := gitUrl.User.Password()
		if err != nil {
			continue
		}
		credential := Credential{
			Path: gitUrl.Host,
			Auth: http.BasicAuth{
				Username: gitUrl.User.Username(),
				Password: password,
			},
		}
		credentials = append(credentials, credential)
	}

	return credentials
}

func FindAuthForHost(baseUrl string) Credential {
	// Read the ~/.git-credentials file
	gitCreds := credentialParser()

	// Will be nil unless a match is found
	var matchedCred Credential

	// Look for a match for the given host path in the creds file
	for _, gitCred := range gitCreds {
		hasPath := strings.Contains(baseUrl, gitCred.Path)
		if hasPath {
			matchedCred = gitCred
			break
		}
	}

	return matchedCred
}

// removeLocalBranchRefs removes all refs that are local branches
// It returns a slice of references deleted
func removeLocalBranchRefs(gitDirectory string) ([]*plumbing.Reference, error) {
	return removeReferences(
		gitDirectory,
		func(ref *plumbing.Reference) bool {
			return ref.Name().IsBranch()
		},
	)
}

// removeOnlineRemoteRefs removes all refs pointing to the online-upstream
// It returns a slice of references deleted
func removeOnlineRemoteRefs(gitDirectory string) ([]*plumbing.Reference, error) {
	return removeReferences(
		gitDirectory,
		func(ref *plumbing.Reference) bool {
			return strings.HasPrefix(ref.Name().String(), onlineRemoteRefPrefix)
		},
	)
}

// removeHeadCopies removes any refs that aren't HEAD but have the same hash
// It returns a slice of references deleted
func removeHeadCopies(gitDirectory string) ([]*plumbing.Reference, error) {
	message.Debugf("Remove head copies for %s", gitDirectory)
	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		return nil, fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to identify references when getting the repo's head: %w", err)
	}

	headHash := head.Hash().String()
	return removeReferences(
		gitDirectory,
		func(ref *plumbing.Reference) bool {
			// Don't ever remove tags
			return !ref.Name().IsTag() && ref.Hash().String() == headHash
		},
	)
}

// removeReferences removes references based on a provided callback
// removeReferences does not allow you to delete HEAD
// It returns a slice of references deleted
func removeReferences(
	gitDirectory string,
	shouldRemove func(*plumbing.Reference) bool,
) ([]*plumbing.Reference, error) {
	message.Debugf("Remove git references %s", gitDirectory)
	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		return nil, fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	references, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to identify references when getting the repo's references: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to identify head: %w", err)
	}

	var removedRefs []*plumbing.Reference
	err = references.ForEach(func(ref *plumbing.Reference) error {
		refIsNotHeadOrHeadTarget := ref.Name() != plumbing.HEAD && ref.Name() != head.Name()
		// Run shouldRemove inline here to take advantage of short circuit
		// evaluation as to not waste a cycle on HEAD
		if refIsNotHeadOrHeadTarget && shouldRemove(ref) {
			err = repo.Storer.RemoveReference(ref.Name())
			if err != nil {
				return err
			}
			removedRefs = append(removedRefs, ref)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to remove references: %w", err)
	}

	return removedRefs, nil
}

// addRefs adds a provided arbitrary list of references to a repo
// It is intended to be used with references returned by a Remove function
func addRefs(gitDirectory string, refs []*plumbing.Reference) error {
	message.Debugf("Add git refs %s", gitDirectory)
	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	for _, ref := range refs {
		err = repo.Storer.SetReference(ref)
		if err != nil {
			return fmt.Errorf("failed to add references: %w", err)
		}
	}

	return nil
}

// deleteBranchIfExists ensures the provided branch name does not exist
func deleteBranchIfExists(gitDirectory string, branchName plumbing.ReferenceName) error {
	message.Debugf("Delete branch %s for %s if it exists", branchName.String(), gitDirectory)

	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		return fmt.Errorf("not a valid git repo or unable to open: %w", err)
	}

	// Deletes the branch by name
	err = repo.DeleteBranch(branchName.Short())
	if err != nil && err != git.ErrBranchNotFound {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	// Delete reference too
	err = repo.Storer.RemoveReference(branchName)
	if err != nil && err != git.ErrInvalidReference {
		return fmt.Errorf("failed to delete branch reference: %w", err)
	}

	return nil
}
