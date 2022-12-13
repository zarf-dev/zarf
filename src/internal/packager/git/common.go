// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories
package git

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

type Git struct {
	Server types.GitServerInfo

	Spinner *message.Spinner

	// Target working directory for the git repository
	GitPath string
}

const onlineRemoteName = "online-upstream"
const offlineRemoteName = "offline-downstream"
const onlineRemoteRefPrefix = "refs/remotes/" + onlineRemoteName + "/"

func New(server types.GitServerInfo) *Git {
	return &Git{
		Server: server,
	}
}

func NewWithSpinner(server types.GitServerInfo, spinner *message.Spinner) *Git {
	return &Git{
		Server:  server,
		Spinner: spinner,
	}
}
