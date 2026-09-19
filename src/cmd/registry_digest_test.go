// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestNormalizeImageRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		imageRef string
		want     string
	}{
		{
			name:     "bare org/repo defaults to docker.io and :latest",
			imageRef: "stefanprodan/podinfo",
			want:     "docker.io/stefanprodan/podinfo:latest",
		},
		{
			name:     "bare single-segment name defaults to docker.io/library and :latest",
			imageRef: "alpine",
			want:     "docker.io/library/alpine:latest",
		},
		{
			name:     "explicit tag is preserved",
			imageRef: "stefanprodan/podinfo:6.4.0",
			want:     "docker.io/stefanprodan/podinfo:6.4.0",
		},
		{
			name:     "explicit registry with port and tag is left unchanged",
			imageRef: "127.0.0.1:31999/longhornio/longhorn-manager:v1.11.2",
			want:     "127.0.0.1:31999/longhornio/longhorn-manager:v1.11.2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeImageRef(tt.imageRef)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeImageRefInvalid(t *testing.T) {
	t.Parallel()
	_, err := normalizeImageRef("Not_A_Valid_Image!!!")
	require.Error(t, err)
}

func TestResolveDigest(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	address := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repoRef := fmt.Sprintf("%s/test-repo", address)
	imageRef := repoRef + ":1.0.0"

	seedRepo := &orasRemote.Repository{
		Client:    &auth.Client{Client: http.DefaultClient},
		PlainHTTP: true,
	}
	var err error
	seedRepo.Reference, err = registry.ParseReference(imageRef)
	require.NoError(t, err)

	desc, err := oras.PackManifest(ctx, seedRepo, oras.PackManifestVersion1_1, "application/vnd.test.artifact", oras.PackManifestOptions{})
	require.NoError(t, err)
	_, err = oras.Tag(ctx, seedRepo, desc.Digest.String(), "1.0.0")
	require.NoError(t, err)

	client := &auth.Client{Client: http.DefaultClient}

	tests := []struct {
		name    string
		fullRef bool
		want    string
	}{
		{
			name: "bare digest",
			want: desc.Digest.String(),
		},
		{
			name:    "full-ref prints repo@digest",
			fullRef: true,
			want:    fmt.Sprintf("%s@%s", repoRef, desc.Digest),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := registryConnection{ref: imageRef, client: client}
			var buf bytes.Buffer
			err := resolveDigest(ctx, &buf, conn, false, false, tt.fullRef)
			require.NoError(t, err)
			require.Equal(t, tt.want, strings.TrimSpace(buf.String()))
		})
	}
}

func TestResolveDigestInvalidRepo(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	conn := registryConnection{ref: "not a valid image ref", client: &auth.Client{Client: http.DefaultClient}}
	var buf bytes.Buffer
	err := resolveDigest(ctx, &buf, conn, false, false, false)
	require.Error(t, err)
}

func TestResolveDigestUsesKnownPlainHTTP(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	address := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	imageRef := fmt.Sprintf("%s/plain-http-known:1.0.0", address)

	seedRepo := &orasRemote.Repository{
		Client:    &auth.Client{Client: http.DefaultClient},
		PlainHTTP: true,
	}
	var err error
	seedRepo.Reference, err = registry.ParseReference(imageRef)
	require.NoError(t, err)
	desc, err := oras.PackManifest(ctx, seedRepo, oras.PackManifestVersion1_1, "application/vnd.test.artifact", oras.PackManifestOptions{})
	require.NoError(t, err)
	_, err = oras.Tag(ctx, seedRepo, desc.Digest.String(), "1.0.0")
	require.NoError(t, err)

	// This registry only speaks plain HTTP. Deliberately claim the opposite (known HTTPS) and
	// expect a failure: if resolveDigest instead fell through to probing the (local) host
	// itself, that probe would correctly detect plain HTTP and the call would succeed despite
	// the wrong known value. Failing here proves the known value is what actually gets used.
	conn := registryConnection{
		ref:            imageRef,
		client:         &auth.Client{Client: http.DefaultClient},
		plainHTTPKnown: true,
		plainHTTP:      false,
	}
	var buf bytes.Buffer
	err = resolveDigest(ctx, &buf, conn, false, false, false)
	require.Error(t, err)
}

func TestTarballDigest(t *testing.T) {
	t.Parallel()
	tarballPath := filepath.Join(t.TempDir(), "image.tar")
	img := empty.Image
	wantDigest, err := img.Digest()
	require.NoError(t, err)

	tag, err := name.NewTag("test.local/image:1.0.0")
	require.NoError(t, err)
	require.NoError(t, tarball.WriteToFile(tarballPath, tag, img))

	tests := []struct {
		name string
		tag  string
	}{
		{name: "no tag argument reads the tarball as a single image"},
		{name: "matching tag argument", tag: "test.local/image:1.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := tarballDigest(&buf, tarballPath, tt.tag)
			require.NoError(t, err)
			require.Equal(t, wantDigest.String(), strings.TrimSpace(buf.String()))
		})
	}
}

func TestTarballDigestInvalidPath(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := tarballDigest(&buf, filepath.Join(t.TempDir(), "does-not-exist.tar"), "")
	require.Error(t, err)
}
