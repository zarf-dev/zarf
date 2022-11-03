// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories
package git

import (
	"bufio"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func (g *Git) FindAuthForHost(baseUrl string) Credential {
	// Read the ~/.git-credentials file
	gitCreds := g.credentialParser()

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

func (g *Git) credentialParser() []Credential {
	var credentials []Credential

	homePath, _ := os.UserHomeDir()
	credentialsPath := filepath.Join(homePath, ".git-credentials")
	credentialsFile, _ := os.Open(credentialsPath)

	defer func(credentialsFile *os.File) {
		err := credentialsFile.Close()
		if err != nil {
			message.Debugf("Unable to load an existing git credentials file: %w", err)
		}
	}(credentialsFile)

	scanner := bufio.NewScanner(credentialsFile)
	for scanner.Scan() {
		gitUrl, err := url.Parse(scanner.Text())
		if err != nil {
			continue
		}
		password, _ := gitUrl.User.Password()
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
