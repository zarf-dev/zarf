package git

import (
	"bufio"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/sirupsen/logrus"
)

type Credential struct {
	Path string
	Auth http.BasicAuth
}

func MutateGitUrlsInText(host string, text string) string {
	extractPathRegex := regexp.MustCompilePOSIX(`https?://[^/]+/(.*\.git)`)
	output := extractPathRegex.ReplaceAllStringFunc(text, func(match string) string {
		if strings.Contains(match, "/zarf-git-user/") {
			logrus.WithField("Match", match).Warn("This url seems to have been previously patched.")
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
	logrus.WithFields(logrus.Fields{
		"Old": url,
		"New": output,
	}).Info("Transformed Git URL")
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
	defer credentialsFile.Close()

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

func GetOrCreateZarfSecret() string {
	var gitSecret string

	credentials := FindAuthForHost(config.GetTargetEndpoint())

	if (credentials == Credential{}) {
		gitSecret = CredentialsGenerator()
	} else {
		gitSecret = credentials.Auth.Password
	}

	return gitSecret
}

func CredentialsGenerator() string {

	// Get a random secret for use in the cluster
	gitSecret := utils.RandomString(28)
	credentialsPath := credentialFilePath()

	// Prevent duplicates by purging the git creds file~
	_ = os.Remove(credentialsPath)

	credentialsFile, err := os.OpenFile(credentialsPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to access the git credentials file")
	}
	defer credentialsFile.Close()

	// Needed by zarf to do repo pushes
	zarfUrl := url.URL{
		Scheme: "https",
		User:   url.UserPassword(config.ZarfGitUser, gitSecret),
		Host:   config.GetTargetEndpoint(),
	}

	credentialsText := zarfUrl.String() + "\n"

	// Write the entry to the file
	_, err = credentialsFile.WriteString(credentialsText)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to update the git credentials file")
	}

	// Save the change
	err = credentialsFile.Sync()
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to update the git credentials file")
	}

	return gitSecret
}

// GetTaggedUrl builds a URL of the repo@tag format
// It returns a string of format repo@tag
func GetTaggedUrl(gitUrl string, gitTag string) string {
	return gitUrl + "@" + gitTag
}

// RemoveLocalBranchRefs removes all refs that are local branches
// It returns a slice of references deleted
func RemoveLocalBranchRefs(gitDirectory string) []*plumbing.Reference {
	return removeReferences(
		gitDirectory,
		func(ref *plumbing.Reference) bool {
			return ref.Name().IsBranch()
		},
	)
}

// RemoveOnlineRemoteRefs removes all refs pointing to the online-upstream
// It returns a slice of references deleted
func RemoveOnlineRemoteRefs(gitDirectory string) []*plumbing.Reference {
	return removeReferences(
		gitDirectory,
		func(ref *plumbing.Reference) bool {
			return strings.HasPrefix(ref.Name().String(), onlineRemoteRefPrefix)
		},
	)
}

// RemoveHeadCopies removes any refs that aren't HEAD but have the same hash
// It returns a slice of references deleted
func RemoveHeadCopies(gitDirectory string) []*plumbing.Reference {
	logContext := logrus.WithField("Repo", gitDirectory)
	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Not a valid git repo or unable to open")
	}

	head, err := repo.Head()
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Failed to identify references")
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
) []*plumbing.Reference {
	logContext := logrus.WithField("Repo", gitDirectory)
	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Not a valid git repo or unable to open")
	}

	references, err := repo.References()
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Failed to identify references")
	}

	head, err := repo.Head()
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Failed to identify head")
	}

	removedRefs := []*plumbing.Reference{}
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
		logContext.Debug(err)
		logContext.Fatal("Failed to remove references")
	}

	return removedRefs
}

// AddRefs adds a provided arbitrary list of references to a repo
// It is intended to be used with references returned by a Remove function
func AddRefs(gitDirectory string, refs []*plumbing.Reference) {
	logContext := logrus.WithField("Repo", gitDirectory)
	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Not a valid git repo or unable to open")
	}

	for _, ref := range refs {
		err = repo.Storer.SetReference(ref)
		if err != nil {
			logContext.Debug(err)
			logContext.Fatal("Failed to add references")
		}
	}
}

// DeleteBranchIfExists ensures the provided branch name does not exist
func DeleteBranchIfExists(gitDirectory string, branchName plumbing.ReferenceName) {
	logContext := logrus.WithFields(logrus.Fields{
		"Repo":   gitDirectory,
		"Branch": branchName.String,
	})

	repo, err := git.PlainOpen(gitDirectory)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Not a valid git repo or unable to open")
	}

	// Deletes the branch by name
	err = repo.DeleteBranch(branchName.Short())
	if err != nil && err != git.ErrBranchNotFound {
		logContext.Debug(err)
		logContext.Fatal("Failed to delete branch")
	}

	// Delete reference too
	err = repo.Storer.RemoveReference(branchName)
	if err != nil && err != git.ErrInvalidReference {
		logContext.Debug(err)
		logContext.Fatal("Failed to delete branch reference")
	}

	logContext.Info("Branch deleted")
}
