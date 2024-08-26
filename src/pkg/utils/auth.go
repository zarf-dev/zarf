// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"bufio"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Credential represents authentication for a given host.
type Credential struct {
	Path string
	Auth http.BasicAuth
}

// FindAuthForHost finds the authentication scheme for a given host using .git-credentials then .netrc.
func FindAuthForHost(baseURL string) (*Credential, error) {
	homePath, _ := os.UserHomeDir()

	// Read the ~/.git-credentials file
	credentialsPath := filepath.Join(homePath, ".git-credentials")
	gitCreds, err := credentialParser(credentialsPath)
	if err != nil {
		return nil, err
	}

	// Read the ~/.netrc file
	netrcPath := filepath.Join(homePath, ".netrc")
	netrcCreds, err := netrcParser(netrcPath)
	if err != nil {
		return nil, err
	}

	// Combine the creds together (.netrc second because it could have a default)
	creds := append(gitCreds, netrcCreds...)
	for _, cred := range creds {
		// An empty credPath means that we have reached the default from the .netrc
		hasPath := strings.Contains(baseURL, cred.Path) || cred.Path == ""
		if hasPath {
			return &cred, nil
		}
	}
	return nil, nil
}

// credentialParser parses a user's .git-credentials file to find git creds for hosts.
func credentialParser(path string) ([]Credential, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var credentials []Credential
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		gitURL, err := url.Parse(scanner.Text())
		if err != nil || gitURL.Host == "" {
			continue
		}
		password, _ := gitURL.User.Password()
		credential := Credential{
			Path: gitURL.Host,
			Auth: http.BasicAuth{
				Username: gitURL.User.Username(),
				Password: password,
			},
		}
		credentials = append(credentials, credential)
	}
	return credentials, nil
}

// netrcParser parses a user's .netrc file using the method curl did pre 7.84.0: https://daniel.haxx.se/blog/2022/05/31/netrc-pains/.
func netrcParser(path string) ([]Credential, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var credentials []Credential
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

		for _, token := range tokens {
			if activeCommand != "" {
				// If we are in an active command, process the next token as a value
				activeMachine[activeCommand] = token
				activeCommand = ""
			} else if strings.HasPrefix(token, "#") {
				// If we have entered into a comment, don't process it
				// NOTE: We could use a similar technique to this for spaces in the future
				// by detecting leading " and trailing \.  See top of function for more info
				break
			} else {
				switch token {
				case "machine":
					// If the token is the start of a machine, append the last machine (if exists) and make a new one
					if activeMachine != nil {
						credentials = appendNetrcMachine(activeMachine, credentials)
					}
					activeMachine = map[string]string{}
					activeCommand = token
				case "macdef":
					// If the token is the start of a macro, enter macro mode
					activeMacro = true
					activeCommand = token
				case "login", "password", "account":
					// If the token is a regular command, set the now active command
					activeCommand = token
				case "default":
					// If the token is the default machine, append the last machine (if exists) and make a default one
					if activeMachine != nil {
						credentials = appendNetrcMachine(activeMachine, credentials)
					}
					activeMachine = map[string]string{"machine": ""}
				}
			}
		}
	}
	// Append the last machine (if exists) at the end of the file
	if activeMachine != nil {
		credentials = appendNetrcMachine(activeMachine, credentials)
	}
	return credentials, nil
}

func appendNetrcMachine(machine map[string]string, credentials []Credential) []Credential {
	credential := Credential{
		Path: machine["machine"],
		Auth: http.BasicAuth{
			Username: machine["login"],
			Password: machine["password"],
		},
	}

	return append(credentials, credential)
}
