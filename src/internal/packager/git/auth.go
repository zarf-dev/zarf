// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories
package git

import (
	"bufio"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// FindAuthForHost finds the authentication scheme for a given host using .git-credentials then .netrc
func (g *Git) FindAuthForHost(baseUrl string) Credential {
	homePath, _ := os.UserHomeDir()

	// Read the ~/.git-credentials file
	credentialsPath := filepath.Join(homePath, ".git-credentials")
	credentialsFile, _ := os.Open(credentialsPath)
	gitCreds := g.credentialParser(credentialsFile)

	// Read the ~/.netrc file
	netrcPath := filepath.Join(homePath, ".netrc")
	netrcFile, _ := os.Open(netrcPath)
	netrcCreds := g.netrcParser(netrcFile)

	// Combine the creds together (.netrc second because it could have a default)
	creds := append(gitCreds, netrcCreds...)

	// Will be nil unless a match is found
	var matchedCred Credential

	// Look for a match for the given host path in the creds file
	for _, cred := range creds {
		// An empty credPath means that we have reached the default from the .netrc
		hasPath := strings.Contains(baseUrl, cred.Path) || cred.Path == ""
		if hasPath {
			matchedCred = cred
			break
		}
	}

	return matchedCred
}

// credentialParser parses a user's .git-credentials file to find git creds for hosts
func (g *Git) credentialParser(file io.ReadCloser) []Credential {
	var credentials []Credential

	defer func(file io.ReadCloser) {
		err := file.Close()
		if err != nil {
			message.Debugf("Unable to load an existing git credentials file: %s", err.Error())
		}
	}(file)

	scanner := bufio.NewScanner(file)
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

// netrcParser parses a user's .netrc file using the method curl did pre 7.84.0: https://daniel.haxx.se/blog/2022/05/31/netrc-pains/
func (g *Git) netrcParser(file io.ReadCloser) []Credential {
	var credentials []Credential

	defer func(file io.ReadCloser) {
		err := file.Close()
		if err != nil {
			message.Debugf("Unable to load an existing netrc file: %s", err.Error())
		}
	}(file)

	scanner := bufio.NewScanner(file)

	activeMacro := false
	activeCommand := ""
	var activeMachine map[string]string

	for scanner.Scan() {
		line := scanner.Text()

		// If we are in a macro block, continue
		if activeMacro {
			if line == "" {
				activeMacro = false
			}
			continue
		}

		// Prepare our line to be tokenized
		line = strings.ReplaceAll(line, "\t", " ")
		line = strings.TrimSpace(line)

		tokens := strings.Split(line, " ")

		// TODO: (@WSTARR) Remove this when finished
		message.Debugf("tokens: '%#v'", tokens)

		for _, token := range tokens {
			if activeCommand != "" {
				// If we are in an active command, process the next token as a value
				activeMachine[activeCommand] = token
				activeCommand = ""
			} else if strings.HasPrefix(token, "#") {
				// If we have entered into a comment, don't process it
				break
			} else if token == "machine" {
				// If the token is the start of a machine, append the last machine (if exists) and make a new one
				if activeMachine != nil {
					credentials = appendNetrcMachine(activeMachine, credentials)
				}
				activeMachine = map[string]string{}
				activeCommand = token
			} else if token == "macdef" {
				// If the token is the start of a macro, enter macro mode
				activeMacro = true
				activeCommand = token
			} else if token == "login" || token == "password" || token == "account" {
				// If the token is a regular command, set the now active command
				activeCommand = token
			} else if token == "default" {
				// If the token is the default machine, append the last machine (if exists) and make a default one
				if activeMachine != nil {
					credentials = appendNetrcMachine(activeMachine, credentials)
				}
				activeMachine = map[string]string{"machine": ""}
			}
		}
	}

	// Append the last machine (if exists) at the end of the file
	if activeMachine != nil {
		credentials = appendNetrcMachine(activeMachine, credentials)
	}

	return credentials
}

func appendNetrcMachine(machine map[string]string, credentials []Credential) []Credential {
	credential := Credential{
		Path: machine["machine"],
		Auth: http.BasicAuth{
			Username: machine["login"],
			Password: machine["password"],
		},
	}

	// TODO: (@WSTARR) Remove this when finished
	message.Debugf("credential: '%#v'", credential)

	return append(credentials, credential)
}
