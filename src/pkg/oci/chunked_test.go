// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestChunkedPush(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		data         string
		minimum      string
		patches      []string
		maxPut       int64
		patchStatus  int
		deleteStatus int
	}{
		{name: "partial final chunk", data: "abcdefghij", patches: []string{"0-3", "4-7", "8-9"}},
		{name: "exact multiple", data: "abcdefgh", patches: []string{"0-3", "4-7"}},
		{name: "one chunk stays monolithic", data: "abcd", maxPut: 4},
		{name: "empty blob stays monolithic", data: ""},
		{name: "registry minimum", data: "abcdefghij", minimum: "8", maxPut: 10},
		{name: "PATCH unsupported", data: "abcdefghij", maxPut: 10, patchStatus: http.StatusMethodNotAllowed},
		{name: "PATCH not implemented", data: "abcdefghij", maxPut: 10, patchStatus: http.StatusNotImplemented},
		{name: "fallback despite DELETE 405", data: "abcdefghij", maxPut: 10, patchStatus: http.StatusMethodNotAllowed, deleteStatus: http.StatusMethodNotAllowed},
		{name: "fallback despite DELETE 404", data: "abcdefghij", maxPut: 10, patchStatus: http.StatusNotImplemented, deleteStatus: http.StatusNotFound},
		{name: "minimum fallback despite DELETE failure", data: "abcdefghij", minimum: "8", maxPut: 10, deleteStatus: http.StatusMethodNotAllowed},
		{name: "invalid minimum ignored", data: "abcdefgh", minimum: "invalid", patches: []string{"0-3", "4-7"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var uploaded bytes.Buffer
			var ranges []string
			var putSize int64
			var methods []string
			desc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, []byte(tc.data))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method)
				// Exercise authentication replay on every request, including PATCH.
				if user, pass, ok := r.BasicAuth(); !ok || user != "user" || pass != "pass" {
					w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				switch r.Method {
				case http.MethodPost:
					w.Header().Set("Location", "/v2/repo/blobs/uploads/session?state=0")
					w.Header().Set("OCI-Chunk-Min-Length", tc.minimum)
					w.WriteHeader(http.StatusAccepted)
				case http.MethodPatch:
					if tc.patchStatus != 0 {
						w.WriteHeader(tc.patchStatus)
						return
					}
					assert.Equal(t, fmt.Sprint(len(ranges)), r.URL.Query().Get("state"))
					ranges = append(ranges, r.Header.Get("Content-Range"))
					assert.LessOrEqual(t, r.ContentLength, int64(4))
					assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
					_, err := io.Copy(&uploaded, r.Body)
					assert.NoError(t, err)
					w.Header().Set("Location", fmt.Sprintf("?state=%d", len(ranges)))
					w.Header().Set("Range", fmt.Sprintf("0-%d", uploaded.Len()-1))
					w.WriteHeader(http.StatusAccepted)
				case http.MethodPut:
					putSize = r.ContentLength
					assert.LessOrEqual(t, putSize, tc.maxPut)
					assert.Equal(t, desc.Digest.String(), r.URL.Query().Get("digest"))
					_, err := io.Copy(&uploaded, r.Body)
					assert.NoError(t, err)
					assert.Equal(t, tc.data, uploaded.String())
					w.WriteHeader(http.StatusCreated)
				case http.MethodDelete:
					if tc.deleteStatus != 0 {
						w.WriteHeader(tc.deleteStatus)
						return
					}
					w.WriteHeader(http.StatusNoContent)
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			defer server.Close()
			repo, err := remote.NewRepository(strings.TrimPrefix(server.URL, "http://") + "/repo")
			require.NoError(t, err)
			repo.PlainHTTP = true
			repo.Client = &auth.Client{Client: server.Client(), Credential: auth.StaticCredential(repo.Reference.Registry, auth.Credential{Username: "user", Password: "pass"})}
			target := &chunkedTarget{Target: repo, repo: repo, chunkSize: 4}
			require.NoError(t, target.Push(t.Context(), desc, strings.NewReader(tc.data)))
			require.Equal(t, tc.patches, ranges)
			require.Equal(t, tc.maxPut, putSize)
			require.Contains(t, methods, http.MethodPut)
		})
	}
}

func TestChunkedPushFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		fail       string
		body       string
		wantErr    string
		canceled   bool
		failStatus int
	}{
		{name: "POST rejected", fail: http.MethodPost, body: "abcdefgh", wantErr: "413"},
		{name: "PATCH rejected", fail: http.MethodPatch, body: "abcdefgh", wantErr: "413", canceled: true},
		{name: "PUT rejected", fail: http.MethodPut, body: "abcdefgh", wantErr: "413", canceled: true},
		{name: "PATCH bad request is not unsupported", fail: http.MethodPatch, failStatus: http.StatusBadRequest, body: "abcdefgh", wantErr: "400", canceled: true},
		{name: "PATCH missing session is not unsupported", fail: http.MethodPatch, failStatus: http.StatusNotFound, body: "abcdefgh", wantErr: "404", canceled: true},
		{name: "short reader", body: "abc", wantErr: "unexpected EOF", canceled: true},
		{name: "unsafe PATCH location", fail: "location", body: "abcdefgh", wantErr: "registry origin", canceled: true},
		{name: "incorrect stored range", fail: "range", body: "abcdefgh", wantErr: "unexpected upload range", canceled: true},
		{name: "canceled upload", fail: "cancel", body: "abcdefgh", wantErr: "context canceled", canceled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var canceled bool
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == tc.fail {
					status := tc.failStatus
					if status == 0 {
						status = http.StatusRequestEntityTooLarge
					}
					w.WriteHeader(status)
					return
				}
				switch r.Method {
				case http.MethodPost, http.MethodPatch:
					_, err := io.Copy(io.Discard, r.Body)
					assert.NoError(t, err)
					w.Header().Set("Location", "/v2/repo/blobs/uploads/session")
					if r.Method == http.MethodPatch && tc.fail == "location" {
						w.Header().Set("Location", "http://attacker.invalid/upload")
					}
					if r.Method == http.MethodPatch && tc.fail == "range" {
						w.Header().Set("Range", "0-99")
					}
					if r.Method == http.MethodPatch && tc.fail == "cancel" {
						cancel()
					}
					w.WriteHeader(http.StatusAccepted)
				case http.MethodDelete:
					canceled = true
					w.WriteHeader(http.StatusNoContent)
				default:
					w.WriteHeader(http.StatusCreated)
				}
			}))
			defer server.Close()
			repo, err := remote.NewRepository(strings.TrimPrefix(server.URL, "http://") + "/repo")
			require.NoError(t, err)
			repo.PlainHTTP = true
			target := &chunkedTarget{Target: repo, repo: repo, chunkSize: 4}
			desc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, []byte("abcdefgh"))
			err = target.Push(ctx, desc, strings.NewReader(tc.body))
			require.ErrorContains(t, err, tc.wantErr)
			require.Equal(t, tc.canceled, canceled)
		})
	}
}

func TestChunkedPushManifest(t *testing.T) {
	t.Parallel()
	for _, mediaType := range []string{ocispec.MediaTypeImageManifest, ocispec.MediaTypeImageIndex, "application/vnd.docker.distribution.manifest.v2+json", "application/custom.manifest"} {
		t.Run(mediaType, func(t *testing.T) {
			t.Parallel()
			data := []byte(`{"schemaVersion":2}`)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				assert.Contains(t, r.URL.Path, "/manifests/")
				assert.Equal(t, mediaType, r.Header.Get("Content-Type"))
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.Equal(t, data, body)
				w.WriteHeader(http.StatusCreated)
			}))
			defer server.Close()
			repo, err := remote.NewRepository(strings.TrimPrefix(server.URL, "http://") + "/repo")
			require.NoError(t, err)
			repo.PlainHTTP = true
			require.NoError(t, repo.SetReferrersCapability(true))
			if mediaType == "application/custom.manifest" {
				repo.ManifestMediaTypes = []string{mediaType}
			}
			target := &chunkedTarget{Target: repo, repo: repo, chunkSize: 4}
			require.NoError(t, target.Push(t.Context(), content.NewDescriptorFromBytes(mediaType, data), bytes.NewReader(data)))
		})
	}
}

func TestUploadLocation(t *testing.T) {
	t.Parallel()
	origin, err := url.Parse("https://registry.example:443/v2/repo/blobs/uploads/")
	require.NoError(t, err)
	for _, location := range []string{"/next", "?state=1", "https://registry.example/next", "http://registry.example/next", "https://attacker.example/next", "https://registry.example:444/next", "https://user:pass@registry.example/next", ""} {
		t.Run(location, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, origin.String(), nil)
			require.NoError(t, err)
			resp := &http.Response{Request: req, Header: http.Header{"Location": []string{location}}}
			_, err = uploadLocation(resp, origin)
			if location == "/next" || location == "?state=1" || location == "https://registry.example/next" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
