// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.package utils
package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/require"
)

func TestCredentialParser(t *testing.T) {
	t.Parallel()

	data := `https://wayne:password@github.com/
bad line
<really bad="line"/>
https://wayne:p%40ss%20word%2520@zarf.dev
http://google.com`
	path := filepath.Join(t.TempDir(), "file")
	err := os.WriteFile(path, []byte(data), 0o644)
	require.NoError(t, err)

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

	gitCredentials, err := credentialParser(path)
	require.NoError(t, err)
	require.Equal(t, expectedCreds, gitCredentials)
}

func TestNetRCParser(t *testing.T) {
	t.Parallel()

	data := `# top of file comment
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
	password password`
	path := filepath.Join(t.TempDir(), "file")
	err := os.WriteFile(path, []byte(data), 0o644)
	require.NoError(t, err)

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

	netrcCredentials, err := netrcParser(path)
	require.NoError(t, err)
	require.Equal(t, expectedCreds, netrcCredentials)
}
