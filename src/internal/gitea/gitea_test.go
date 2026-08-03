// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package gitea

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	c, err := NewClient("https://example.com", "foo", "bar")
	require.NoError(t, err)
	require.Equal(t, "https", c.endpoint.Scheme)
	require.Equal(t, "foo", c.username)
	require.Equal(t, "bar", c.password)
}

func TestUpdateGitUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "success", status: http.StatusOK, wantErr: false},
		{name: "unauthorized", status: http.StatusUnauthorized, wantErr: true},
		{name: "not found", status: http.StatusNotFound, wantErr: true},
		{name: "server error", status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			c, err := NewClient(srv.URL, "admin", "password")
			require.NoError(t, err)

			err = c.UpdateGitUser(context.Background(), "zarf-git-user", "new-password")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
