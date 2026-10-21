// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestNewRemoteWithOptionsCredentialFunc(t *testing.T) {
	t.Parallel()

	credential := auth.Credential{Username: "registry-user", Password: "registry-password"}
	manifest := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[]}`)
	manifestDigest := digest.FromBytes(manifest)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/test/manifests/latest" {
			http.NotFound(w, r)
			return
		}
		username, password, ok := r.BasicAuth()
		if !ok || username != credential.Username || password != credential.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="test-registry"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
		w.Header().Set("Content-Length", strconv.Itoa(len(manifest)))
		w.Header().Set("Docker-Content-Digest", manifestDigest.String())
		if r.Method == http.MethodGet {
			if _, err := w.Write(manifest); err != nil {
				t.Error(err)
			}
		}
	}))
	t.Cleanup(server.Close)
	host := strings.TrimPrefix(server.URL, "https://")
	transport, ok := server.Client().Transport.(*http.Transport)
	require.True(t, ok)

	remote, err := NewRemoteWithOptions(context.Background(), host+"/test:latest", oci.PlatformForArch("amd64"), RemoteClientOptions{
		CredentialFunc: auth.StaticCredential(host, credential),
		Transport:      transport,
	})
	require.NoError(t, err)

	descriptor, reader, err := remote.Repo().FetchReference(context.Background(), "latest")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, manifest, content)
	require.Equal(t, manifestDigest, descriptor.Digest)
}
