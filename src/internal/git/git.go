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

// URLWithRef returns a Git URL that selects ref. A nil ref preserves url for
// v1alpha1 repositories, which embed their reference in the URL.
// FIXME: error when there is both a ref in the url and an actual ref
func URLWithRef(url string, ref *api.GitRef) string {
	if ref == nil {
		return url
	}
	if baseURL, _, err := transform.GitURLSplitRef(url); err == nil {
		url = baseURL
	}
	switch {
	case ref.Tag != "":
		return url + "@" + ref.Tag
	case ref.Branch != "":
		return url + "@refs/heads/" + ref.Branch
	case ref.Commit != "":
		return url + "@" + ref.Commit
	default:
		return url
	}
}
