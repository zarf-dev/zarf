// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package git contains functions for interacting with git repositories
package git

import (
	"testing"

	test "github.com/defenseunicorns/zarf/src/test/mocks"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
)

func TestCredentialParser(t *testing.T) {
	g := New(types.GitServerInfo{})

	credentialsFile := &test.MockReadCloser{
		MockData: []byte(
			`https://wayne:password@github.com/
bad line
<really bad=\"line\"/>
https://wayne:p%40ss%20word%2520@zarf.dev
http://google.com`,
		),
	}

	expectedCreds := []Credential{
		{
			Path: "github.com",
			Auth: http.BasicAuth{
				Username: "wayne",
				Password: "password",
			},
		},
		{
			Path: "zarf.dev",
			Auth: http.BasicAuth{
				Username: "wayne",
				Password: "p@ss word%20",
			},
		},
		{
			Path: "google.com",
			Auth: http.BasicAuth{
				Username: "",
				Password: "",
			},
		},
	}

	gitCredentials := g.credentialParser(credentialsFile)
	assert.Equal(t, expectedCreds, gitCredentials)
}

func TestNetRCParser(t *testing.T) {
	g := New(types.GitServerInfo{})

	netrcFile := &test.MockReadCloser{
		MockData: []byte(
			`# top of file comment
machine github.com
	login wayne
    password password

 machine zarf.dev login wayne password p@s#sword%20 

macdef macro-name
touch file
 echo "I am a script and can do anything password fun or login info yay!"

machine google.com #comment password fun and login info!

default
  login anonymous
	password password`,
		),
	}

	expectedCreds := []Credential{
		{
			Path: "github.com",
			Auth: http.BasicAuth{
				Username: "wayne",
				Password: "password",
			},
		},
		{
			Path: "zarf.dev",
			Auth: http.BasicAuth{
				Username: "wayne",
				Password: "p@s#sword%20",
			},
		},
		{
			Path: "google.com",
			Auth: http.BasicAuth{
				Username: "",
				Password: "",
			},
		},
		{
			Path: "",
			Auth: http.BasicAuth{
				Username: "anonymous",
				Password: "password",
			},
		},
	}

	netrcCredentials := g.netrcParser(netrcFile)
	assert.Equal(t, expectedCreds, netrcCredentials)
}
