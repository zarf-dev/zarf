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

func TestMatchCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		creds    []Credential
		expected *Credential
	}{
		{
			name: "exact host match",
			url:  "https://github.com/repo.git",
			creds: []Credential{
				{Path: "github.com", Auth: http.BasicAuth{Username: "user", Password: "token"}},
			},
			expected: &Credential{Path: "github.com", Auth: http.BasicAuth{Username: "user", Password: "token"}},
		},
		{
			name: "host in path does not match",
			url:  "https://evil.com/github.com/repo.git",
			creds: []Credential{
				{Path: "github.com", Auth: http.BasicAuth{Username: "user", Password: "token"}},
			},
			expected: nil,
		},
		{
			name: "host as subdomain prefix does not match",
			url:  "https://github.com.evil.com/repo.git",
			creds: []Credential{
				{Path: "github.com", Auth: http.BasicAuth{Username: "user", Password: "token"}},
			},
			expected: nil,
		},
		{
			name: "unrelated host falls through to default",
			url:  "https://example.com/repo.git",
			creds: []Credential{
				{Path: "github.com", Auth: http.BasicAuth{Username: "user", Password: "token"}},
				{Path: "", Auth: http.BasicAuth{Username: "anonymous", Password: "pass"}},
			},
			expected: &Credential{Path: "", Auth: http.BasicAuth{Username: "anonymous", Password: "pass"}},
		},
		{
			name: "no match and no default",
			url:  "https://example.com/repo.git",
			creds: []Credential{
				{Path: "github.com", Auth: http.BasicAuth{Username: "user", Password: "token"}},
			},
			expected: nil,
		},
		{
			name: "host with port matches credential with same port",
			url:  "https://registry.example.com:5000/repo",
			creds: []Credential{
				{Path: "registry.example.com:5000", Auth: http.BasicAuth{Username: "user", Password: "token"}},
			},
			expected: &Credential{Path: "registry.example.com:5000", Auth: http.BasicAuth{Username: "user", Password: "token"}},
		},
		{
			name: "host with port does not match credential without port",
			url:  "https://registry.example.com:5000/repo",
			creds: []Credential{
				{Path: "registry.example.com", Auth: http.BasicAuth{Username: "user", Password: "token"}},
			},
			expected: nil,
		},
		{
			name: "host without port does not match credential with port",
			url:  "https://registry.example.com/repo",
			creds: []Credential{
				{Path: "registry.example.com:5000", Auth: http.BasicAuth{Username: "user", Password: "token"}},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := matchCredential(tt.url, tt.creds)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchCredentialInvalidURL(t *testing.T) {
	t.Parallel()
	_, err := matchCredential("://invalid", nil)
	require.Error(t, err)
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
