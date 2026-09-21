// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

const onlineRemoteName = "online-upstream"
const offlineRemoteName = "offline-downstream"
const emptyRef = ""

// ParseRef parses the provided ref into a ReferenceName if it's not a hash.
func ParseRef(r string) plumbing.ReferenceName {
	// If not a full ref, assume it's a tag at this point.
	if !plumbing.IsHash(r) && !strings.HasPrefix(r, "refs/") {
		r = fmt.Sprintf("refs/tags/%s", r)
	}
	// Set the reference name to the provided ref.
	return plumbing.ReferenceName(r)
}

// repositoryAddress returns a Git URL that selects the repository reference.
func repositoryAddress(repository api.Repository) (string, error) {
	if repository.Ref == nil {
		return repository.URL, nil
	}
	url, _, err := transform.GitURLSplitRef(repository.URL)
	if err != nil {
		return "", err
	}
	switch {
	case repository.Ref.Tag != "":
		return url + "@" + repository.Ref.Tag, nil
	case repository.Ref.Branch != "":
		return url + "@refs/heads/" + repository.Ref.Branch, nil
	case repository.Ref.Commit != "":
		return url + "@" + repository.Ref.Commit, nil
	default:
		return url, nil
	}
}

// repositoryLayoutAddress returns the URL used to derive the repository directory name.
func repositoryLayoutAddress(repository api.Repository) (string, error) {
	if repository.LegacyURL != "" {
		return repository.LegacyURL, nil
	}
	return repositoryAddress(repository)
}
