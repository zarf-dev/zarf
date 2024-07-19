// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories.
package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/types"
)

// Git is the main struct for managing git repositories.
type Git struct {
	// Server is the git server configuration.
	Server types.GitServerInfo
	// Spinner is an optional spinner to use for long running operations.
	Spinner *message.Spinner
	// Target working directory for the git repository.
	GitPath string
}

const onlineRemoteName = "online-upstream"
const offlineRemoteName = "offline-downstream"
const emptyRef = ""

// New creates a new git instance with the provided server config.
func New(server types.GitServerInfo) *Git {
	return &Git{
		Server: server,
	}
}

// NewWithSpinner creates a new git instance with the provided server config and spinner.
func NewWithSpinner(server types.GitServerInfo, spinner *message.Spinner) *Git {
	return &Git{
		Server:  server,
		Spinner: spinner,
	}
}

// ParseRef parses the provided ref into a ReferenceName if it's not a hash.
func ParseRef(r string) plumbing.ReferenceName {
	// If not a full ref, assume it's a tag at this point.
	if !plumbing.IsHash(r) && !strings.HasPrefix(r, "refs/") {
		r = fmt.Sprintf("refs/tags/%s", r)
	}

	// Set the reference name to the provided ref.
	return plumbing.ReferenceName(r)
}
