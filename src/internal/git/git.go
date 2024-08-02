// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
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
