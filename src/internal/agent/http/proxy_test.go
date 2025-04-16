// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package http

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/types"
)

func TestProxyRequestTransform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		target       string
		state        *state.State
		expectedPath string
	}{
		{
			name:   "basic request",
			target: "http://example.com/zarf-3xx-no-transform/test",
			state: &state.State{
				ArtifactServer: types.ArtifactServerInfo{
					PushUsername: "push-user",
					PushToken:    "push-token",
				},
			},
			expectedPath: "/test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req.Header.Set("Accept-Encoding", "foo")
			err := proxyRequestTransform(req, tt.state)
			require.NoError(t, err)

			require.Empty(t, req.Header.Get("Accept-Encoding"))

			username, password, ok := req.BasicAuth()
			require.True(t, ok)
			require.Equal(t, tt.state.ArtifactServer.PushUsername, username)
			require.Equal(t, tt.state.ArtifactServer.PushToken, password)

			require.Equal(t, tt.expectedPath, req.URL.Path)
		})
	}
}

func TestGetTLSScheme(t *testing.T) {
	t.Parallel()

	scheme := getTLSScheme(nil)
	require.Equal(t, "http://", scheme)
	scheme = getTLSScheme(&tls.ConnectionState{})
	require.Equal(t, "https://", scheme)
}

func TestGetRequestURI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		query    string
		fragment string
		expected string
	}{
		{
			name:     "basic",
			path:     "/foo",
			query:    "",
			fragment: "",
			expected: "/foo",
		},
		{
			name:     "query",
			path:     "/foo",
			query:    "key=value",
			fragment: "",
			expected: "/foo?key=value",
		},
		{
			name:     "fragment",
			path:     "/foo",
			query:    "",
			fragment: "bar",
			expected: "/foo#bar",
		},
		{
			name:     "query and fragment",
			path:     "/foo",
			query:    "key=value",
			fragment: "bar",
			expected: "/foo?key=value#bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uri := getRequestURI(tt.path, tt.query, tt.fragment)
			require.Equal(t, tt.expected, uri)
		})
	}
}

func TestUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		userAgent   string
		expectedGit bool
		expectedPip bool
		expectedNpm bool
	}{
		{
			name:        "unknown user agent",
			userAgent:   "Firefox",
			expectedGit: false,
			expectedPip: false,
			expectedNpm: false,
		},
		{
			name:        "git user agent",
			userAgent:   "git/2.0.0",
			expectedGit: true,
			expectedPip: false,
			expectedNpm: false,
		},
		{
			name:        "pip user agent",
			userAgent:   "pip/1.2.3",
			expectedGit: false,
			expectedPip: true,
			expectedNpm: false,
		},
		{
			name:        "twine user agent",
			userAgent:   "twine/1.8.1",
			expectedGit: false,
			expectedPip: true,
			expectedNpm: false,
		},
		{
			name:        "npm user agent",
			userAgent:   "npm/1.0.0",
			expectedGit: false,
			expectedPip: false,
			expectedNpm: true,
		},
		{
			name:        "pnpm user agent",
			userAgent:   "pnpm/1.0.0",
			expectedGit: false,
			expectedPip: false,
			expectedNpm: true,
		},
		{
			name:        "yarn user agent",
			userAgent:   "yarn/1.0.0",
			expectedGit: false,
			expectedPip: false,
			expectedNpm: true,
		},
		{
			name:        "bun user agent",
			userAgent:   "bun/1.0.0",
			expectedGit: false,
			expectedPip: false,
			expectedNpm: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expectedGit, isGitUserAgent(tt.userAgent))
			require.Equal(t, tt.expectedPip, isPipUserAgent(tt.userAgent))
			require.Equal(t, tt.expectedNpm, isNpmUserAgent(tt.userAgent))
		})
	}
}
