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
			"https://wayne:password@github.com/\n" +
				"https://wayne:p%40ss%20word@zarf.dev\n" +
				"http://google.com",
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
				Password: "p@ss word",
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
