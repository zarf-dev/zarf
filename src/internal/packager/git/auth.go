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
	netrcCreds := g.netrcParser()

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

func (g *Git) credentialParser() []Credential {
	var credentials []Credential

	homePath, _ := os.UserHomeDir()
	credentialsPath := filepath.Join(homePath, ".git-credentials")
	credentialsFile, _ := os.Open(credentialsPath)

	defer func(credentialsFile *os.File) {
		err := credentialsFile.Close()
		if err != nil {
			message.Debugf("Unable to load an existing git credentials file: %s", err.Error())
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

func (g *Git) netrcParser() []Credential {
	var credentials []Credential

	homePath, _ := os.UserHomeDir()
	credentialsPath := filepath.Join(homePath, ".netrc")
	credentialsFile, _ := os.Open(credentialsPath)

	defer func(credentialsFile *os.File) {
		err := credentialsFile.Close()
		if err != nil {
			message.Debugf("Unable to load an existing netrc file: %s", err.Error())
		}
	}(credentialsFile)

	scanner := bufio.NewScanner(credentialsFile)

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
			if token == "#" {
				// If we have entered into a comment, don't process it
				break
			} else if activeCommand != "" {
				// If we are in an active command, process the next token as a value
				activeMachine[activeCommand] = token
				activeCommand = ""
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
