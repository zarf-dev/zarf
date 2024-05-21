// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package transform provides helper functions to transform URLs to airgap equivalents
package transform

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/defenseunicorns/pkg/helpers"
)

// For further explanation: https://regex101.com/r/YxpfhC/5
var gitURLRegex = regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)\/(?P<repo>[\w\-\.]+?)?(?P<git>\.git)?(\/)?(?P<atRef>@(?P<force>\+)?(?P<ref>[\/\+\w\-\.]+))?(?P<gitPath>\/(?P<gitPathId>info\/.*|git-upload-pack|git-receive-pack))?$`)

// MutateGitURLsInText changes the gitURL hostname to use the repository Zarf is configured to use.
func MutateGitURLsInText(logger Log, targetBaseURL string, text string, pushUser string) string {
	extractPathRegex := regexp.MustCompile(`[a-z]+:\/\/[^\/]+\/(.*\.git)`)
	output := extractPathRegex.ReplaceAllStringFunc(text, func(match string) string {
		output, err := GitURL(targetBaseURL, match, pushUser)
		if err != nil {
			logger("Unable to transform the git url, using the original url we have: %s", match)
			return match
		}
		return output.String()
	})
	return output
}

// GitURLSplitRef takes a git url and returns a separated source url and zarf reference.
func GitURLSplitRef(sourceURL string) (string, string, error) {
	get, err := helpers.MatchRegex(gitURLRegex, sourceURL)

	if err != nil {
		return "", "", fmt.Errorf("unable to get extract the source url and ref from the url %s", sourceURL)
	}

	gitURLNoRef := fmt.Sprintf("%s%s/%s%s", get("proto"), get("hostPath"), get("repo"), get("git"))
	refPlain := get("ref")

	return gitURLNoRef, refPlain, nil
}

// GitURLtoFolderName takes a git url and returns the folder name for the repo in the Zarf package.
func GitURLtoFolderName(sourceURL string) (string, error) {
	get, err := helpers.MatchRegex(gitURLRegex, sourceURL)

	if err != nil {
		// Unable to find a substring match for the regex
		return "", fmt.Errorf("unable to get extract the folder name from the url %s", sourceURL)
	}

	repoName := get("repo")
	// NOTE: For folders we use the full URL (without any protocol stuff) so that different refs are kept in different folders on disk to avoid conflicts
	// Add crc32 hash of the repoName to the end of the repo
	gitURL := fmt.Sprintf("%s%s/%s%s%s", get("proto"), get("hostPath"), get("repo"), get("git"), get("atRef"))

	checksum := helpers.GetCRCHash(gitURL)

	newRepoName := fmt.Sprintf("%s-%d", repoName, checksum)

	return newRepoName, nil
}

// GitURLtoRepoName takes a git url and returns the name of the repo in the remote airgap repository.
func GitURLtoRepoName(sourceURL string) (string, error) {
	get, err := helpers.MatchRegex(gitURLRegex, sourceURL)

	if err != nil {
		// Unable to find a substring match for the regex
		return "", fmt.Errorf("unable to get extract the repo name from the url %s", sourceURL)
	}

	repoName := get("repo")
	// NOTE: We remove the .git and protocol so that https://zarf.dev/repo.git and http://zarf.dev/repo
	// resolve to the same repo (as they would in real life)
	sanitizedURL := fmt.Sprintf("%s/%s", get("hostPath"), repoName)

	// Add crc32 hash of the repoName to the end of the repo
	checksum := helpers.GetCRCHash(sanitizedURL)

	newRepoName := fmt.Sprintf("%s-%d", repoName, checksum)

	return newRepoName, nil
}

// GitURL takes a base URL, a source url and a username and returns a Zarf-compatible url.
func GitURL(targetBaseURL string, sourceURL string, pushUser string) (*url.URL, error) {
	repoName, err := GitURLtoRepoName(sourceURL)
	if err != nil {
		return nil, err
	}

	// Get the full path
	matches := gitURLRegex.FindStringSubmatch(sourceURL)
	idx := gitURLRegex.SubexpIndex

	if len(matches) == 0 {
		// Unable to find a substring match for the regex
		return nil, fmt.Errorf("unable to extract the airgap target url from the url %s", sourceURL)
	}

	output := fmt.Sprintf("%s/%s/%s%s%s", targetBaseURL, pushUser, repoName, matches[idx("git")], matches[idx("gitPath")])
	return url.Parse(output)
}
