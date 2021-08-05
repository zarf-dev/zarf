package git

import (
	"bufio"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
)

type credential struct {
	path string
	auth http.BasicAuth
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

func credentialParser() []credential {
	credentialsPath := credentialFilePath()
	var credentials []credential

	credentialsFile, err := os.Open(credentialsPath)
	if err != nil {
		logrus.Info(credentialsPath + " file not found")
	}
	defer credentialsFile.Close()

	scanner := bufio.NewScanner(credentialsFile)
	for scanner.Scan() {
		gitUrl, err := url.Parse(scanner.Text())
		password, _ := gitUrl.User.Password()
		if err != nil {
			continue
		}
		credential := credential{
			path: gitUrl.Host,
			auth: http.BasicAuth{
				Username: gitUrl.User.Username(),
				Password: password,
			},
		}
		credentials = append(credentials, credential)
	}

	if err := scanner.Err(); err != nil {
		logrus.Warn("Error parsing git credentials file")
	}

	return credentials
}

func findAuthForHost(baseUrl string) credential {
	// Read the ~/.git-credentials file
	gitCreds := credentialParser()

	// Will be nil unless a match is found
	var matchedCred credential

	// Look for a match for the given host path in the creds file
	for _, gitCred := range gitCreds {
		hasPath := strings.Contains(baseUrl, gitCred.path)
		if hasPath {
			matchedCred = gitCred
			break
		}
	}

	return matchedCred
}

func CredentialsGenerator(host string, username string, password string) {
	credentialsPath := credentialFilePath()

	// Prevent duplicates by purging the git creds file~
	_ = os.Remove(credentialsPath)

	credentialsFile, err := os.OpenFile(credentialsPath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		logrus.Fatal("Unable to access the git credentials file")
	}
	defer credentialsFile.Close()

	// This allows the git cli access to gitea as root
	customUrl := url.URL{
		Scheme: "https",
		User:   url.UserPassword(username, password),
		Host:   host,
	}
	// Needed by zarf to do repo pushes
	zarfUrl := url.URL{
		Scheme: "https",
		User:   url.UserPassword(username, password),
		Host:   config.ZarfLocal,
	}

	credentialsText := customUrl.String() + "\n" + zarfUrl.String() + "\n"

	// Write the entry to the file
	_, err = credentialsFile.WriteString(credentialsText)
	if err != nil {
		logrus.Fatal("Unable to update the git credentials file")
	}

	// Save the change
	err = credentialsFile.Sync()
	if err != nil {
		logrus.Fatal("Unable to update the git credentials file")
	}
}
