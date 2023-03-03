// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"fmt"
	"regexp"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// For further explanation: https://regex101.com/r/xx8NQe/1.
var gitURLRegex = regexp.MustCompile(`^(?P<proto>[a-z]+:\/\/)(?P<hostPath>.+?)\/(?P<repo>[\w\-\.]+?)(?P<git>\.git)?(?P<atRef>@(?P<force>\+)?(?P<ref>[\/\+\w\-\.]+))?$`)

// MutateGitURLsInText changes the gitURL hostname to use the repository Zarf is configured to use.
func (g *Git) MutateGitURLsInText(text string) string {
	extractPathRegex := regexp.MustCompilePOSIX(`https?://[^/]+/(.*\.git)`)
	output := extractPathRegex.ReplaceAllStringFunc(text, func(match string) string {
		output, err := g.TransformURL(match)
		if err != nil {
			message.Warnf("Unable to transform the git url, using the original url we have: %s", match)
		}
		return output
	})
	return output
}

// TransformURLtoRepoName takes a git url and returns a Zarf-compatible repo name.
func (g *Git) TransformURLtoRepoName(url string) (string, error) {
	get, err := utils.MatchRegex(gitURLRegex, url)

	if err != nil {
		// Unable to find a substring match for the regex
		return "", fmt.Errorf("unable to get extract the repoName from the url %s", url)
	}

	repoName := get("repo")
	// NOTE: We remove the .git and protocol so that https://zarf.dev/repo.git and http://zarf.dev/repo
	// resolve to the same repp (as they would in real life)
	sanitizedURL := fmt.Sprintf("%s/%s", get("hostPath"), repoName)

	// Add crc32 hash of the repoName to the end of the repo
	checksum := utils.GetCRCHash(sanitizedURL)

	newRepoName := fmt.Sprintf("%s-%d", repoName, checksum)

	return newRepoName, nil
}

// TransformURL takes a git url and returns a Zarf-compatible url.
func (g *Git) TransformURL(url string) (string, error) {
	repoName, err := g.TransformURLtoRepoName(url)
	if err != nil {
		return url, err
	}
	output := fmt.Sprintf("%s/%s/%s", g.Server.Address, g.Server.PushUsername, repoName)
	message.Debugf("Rewrite git URL: %s -> %s", url, output)
	return output, nil
}
