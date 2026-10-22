// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestNewOrasRemoteCredentialFunc(t *testing.T) {
	credential := auth.Credential{Username: "registry-user", Password: "registry-password"}
	providerErr := errors.New("credential provider unavailable")
	basicCredential := func(_ context.Context, _ string) (auth.Credential, error) {
		return credential, nil
	}
	emptyCredential := func(_ context.Context, _ string) (auth.Credential, error) {
		return auth.EmptyCredential, nil
	}
	withoutHome := func(t *testing.T, _ string) {
		t.Helper()
		for _, name := range []string{"HOME", "USERPROFILE", "DOCKER_CONFIG"} {
			t.Setenv(name, "")
			require.NoError(t, os.Unsetenv(name))
		}
	}
	withDockerCredentials := func(t *testing.T, host string) {
		t.Helper()
		config, err := json.Marshal(map[string]any{
			"auths": map[string]any{
				host: map[string]string{
					"auth": base64.StdEncoding.EncodeToString([]byte(credential.Username + ":" + credential.Password)),
				},
			},
		})
		require.NoError(t, err)
		writeCredentialTestDockerConfig(t, config)
	}
	tests := []struct {
		name      string
		setup     func(*testing.T, string)
		modifiers []Modifier
		anonymous bool
		wantErr   error
	}{
		{
			name:      "uses explicit credentials without a home directory",
			setup:     withoutHome,
			modifiers: []Modifier{WithCredentialFunc(basicCredential)},
		},
		{
			name:      "acquires an anonymous bearer token without a home directory",
			setup:     withoutHome,
			modifiers: []Modifier{WithCredentialFunc(emptyCredential)},
			anonymous: true,
		},
		{
			name: "ignores malformed Docker configuration for explicit credentials",
			setup: func(t *testing.T, _ string) {
				writeCredentialTestDockerConfig(t, []byte("{invalid Docker configuration"))
			},
			modifiers: []Modifier{WithCredentialFunc(basicCredential)},
		},
		{
			name: "ignores an unavailable Docker credential helper for anonymous access",
			setup: func(t *testing.T, _ string) {
				writeCredentialTestDockerConfig(t, []byte(`{"credsStore":"zarf-test-helper-does-not-exist"}`))
			},
			modifiers: []Modifier{WithCredentialFunc(emptyCredential)},
			anonymous: true,
		},
		{
			name:  "uses Docker credentials when no provider is supplied",
			setup: withDockerCredentials,
		},
		{
			name:      "uses Docker credentials when the provider is nil",
			setup:     withDockerCredentials,
			modifiers: []Modifier{WithCredentialFunc(nil)},
		},
		{
			name:      "does not fall back to Docker when the provider returns empty credentials",
			setup:     withDockerCredentials,
			modifiers: []Modifier{WithCredentialFunc(emptyCredential)},
			wantErr:   auth.ErrBasicCredentialNotFound,
		},
		{
			name:  "does not fall back to Docker when the provider fails",
			setup: withDockerCredentials,
			modifiers: []Modifier{WithCredentialFunc(func(_ context.Context, _ string) (auth.Credential, error) {
				return auth.EmptyCredential, providerErr
			})},
			wantErr: providerErr,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, descriptor := newCredentialTestRegistry(t, credential, tc.anonymous)
			host := strings.TrimPrefix(server.URL, "http://")
			tc.setup(t, host)
			modifiers := append([]Modifier{WithPlainHTTP(true)}, tc.modifiers...)

			remote, err := NewOrasRemote(host+"/credential-tests:latest", ocispec.Platform{}, modifiers...)
			require.NoError(t, err)
			resolved, err := remote.Repo().Resolve(context.Background(), "latest")
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, descriptor, resolved)
		})
	}
}

func TestNewOrasRemoteCredentialFuncUsesNormalizedRegistry(t *testing.T) {
	t.Parallel()

	credential := auth.Credential{Username: "registry-user", Password: "registry-password"}
	server, descriptor := newCredentialTestRegistry(t, credential, false)
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok)
	transport := defaultTransport.Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(transport.CloseIdleConnections)
	var credentialHost string
	remote, err := NewOrasRemote("docker.io/credential-tests:latest", ocispec.Platform{},
		WithPlainHTTP(true), WithTransport(transport),
		WithCredentialFunc(func(_ context.Context, hostport string) (auth.Credential, error) {
			credentialHost = hostport
			return credential, nil
		}))
	require.NoError(t, err)

	resolved, err := remote.Repo().Resolve(context.Background(), "latest")
	require.NoError(t, err)
	require.Equal(t, descriptor, resolved)
	require.Equal(t, "registry-1.docker.io", credentialHost)
	require.Equal(t, "credential-tests", remote.Repo().Reference.Repository)
}

func newCredentialTestRegistry(t *testing.T, credential auth.Credential, anonymous bool) (*httptest.Server, ocispec.Descriptor) {
	t.Helper()
	manifest := []byte(`{"schemaVersion":2,"manifests":[]}`)
	descriptor := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, manifest)
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if anonymous && r.URL.Path == "/token" {
			if r.Header.Get("Authorization") != "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := fmt.Fprint(w, `{"token":"anonymous-registry-token"}`); err != nil {
				t.Errorf("write token response: %v", err)
			}
			return
		}
		if anonymous {
			if r.Header.Get("Authorization") != "Bearer anonymous-registry-token" {
				w.Header().Set("Www-Authenticate", fmt.Sprintf(`Bearer realm=%q,service="registry"`, serverURL+"/token"))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		} else {
			username, password, ok := r.BasicAuth()
			if !ok || username != credential.Username || password != credential.Password {
				w.Header().Set("Www-Authenticate", `Basic realm="registry"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		w.Header().Set("Content-Type", descriptor.MediaType)
		w.Header().Set("Content-Length", strconv.FormatInt(descriptor.Size, 10))
		w.Header().Set("Docker-Content-Digest", descriptor.Digest.String())
		if r.Method != http.MethodHead {
			if _, err := w.Write(manifest); err != nil {
				t.Errorf("write manifest response: %v", err)
			}
		}
	}))
	serverURL = server.URL
	t.Cleanup(server.Close)
	return server, descriptor
}

func writeCredentialTestDockerConfig(t *testing.T, config []byte) {
	t.Helper()
	configDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), config, 0o600))
	t.Setenv("DOCKER_CONFIG", configDir)
}
