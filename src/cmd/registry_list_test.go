// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestListRegistryTags(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	address := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repoRef := fmt.Sprintf("%s/test-repo", address)

	seedRepo := &orasRemote.Repository{
		Client:    &auth.Client{Client: http.DefaultClient},
		PlainHTTP: true,
	}
	var err error
	seedRepo.Reference, err = registry.ParseReference(repoRef)
	require.NoError(t, err)

	desc, err := oras.PackManifest(ctx, seedRepo, oras.PackManifestVersion1_1, "application/vnd.test.artifact", oras.PackManifestOptions{})
	require.NoError(t, err)

	tags := []string{"1.0.0", "2.0.0", "sha256-deadbeef.sig"}
	for _, tag := range tags {
		_, err := oras.Tag(ctx, seedRepo, desc.Digest.String(), tag)
		require.NoError(t, err)
	}

	client := &auth.Client{Client: http.DefaultClient}

	tests := []struct {
		name           string
		fullRef        bool
		omitDigestTags bool
		want           []string
	}{
		{
			name: "default: all tags, bare",
			want: []string{"1.0.0", "2.0.0", "sha256-deadbeef.sig"},
		},
		{
			name:    "full-ref prints repo:tag",
			fullRef: true,
			want: []string{
				repoRef + ":1.0.0",
				repoRef + ":2.0.0",
				repoRef + ":sha256-deadbeef.sig",
			},
		},
		{
			name:           "omit-digest-tags filters sha256- prefixed tags",
			omitDigestTags: true,
			want:           []string{"1.0.0", "2.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := registryConnection{ref: repoRef, client: client}
			var buf bytes.Buffer
			err := listRegistryTags(ctx, &buf, conn, false, false, tt.fullRef, tt.omitDigestTags)
			require.NoError(t, err)
			got := strings.Split(strings.TrimSpace(buf.String()), "\n")
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeRepoRef(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		repoRef string
		want    string
	}{
		{
			name:    "bare org/repo defaults to docker.io",
			repoRef: "stefanprodan/podinfo",
			want:    "docker.io/stefanprodan/podinfo",
		},
		{
			name:    "bare single-segment name defaults to docker.io/library",
			repoRef: "alpine",
			want:    "docker.io/library/alpine",
		},
		{
			name:    "explicit docker.io is left unchanged",
			repoRef: "docker.io/stefanprodan/podinfo",
			want:    "docker.io/stefanprodan/podinfo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeRepoRef(tt.repoRef)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeRepoRefInvalid(t *testing.T) {
	t.Parallel()
	_, err := normalizeRepoRef("Not_A_Valid_Repo!!!")
	require.Error(t, err)
}

func TestListRegistryTagsInvalidRepo(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	conn := registryConnection{ref: "not a valid repo ref", client: &auth.Client{Client: http.DefaultClient}}
	var buf bytes.Buffer
	err := listRegistryTags(ctx, &buf, conn, false, false, false, false)
	require.Error(t, err)
}

func TestListRegistryTagsUsesKnownPlainHTTP(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	address := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repoRef := fmt.Sprintf("%s/plain-http-known", address)

	seedRepo := &orasRemote.Repository{
		Client:    &auth.Client{Client: http.DefaultClient},
		PlainHTTP: true,
	}
	var err error
	seedRepo.Reference, err = registry.ParseReference(repoRef)
	require.NoError(t, err)
	desc, err := oras.PackManifest(ctx, seedRepo, oras.PackManifestVersion1_1, "application/vnd.test.artifact", oras.PackManifestOptions{})
	require.NoError(t, err)
	_, err = oras.Tag(ctx, seedRepo, desc.Digest.String(), "1.0.0")
	require.NoError(t, err)

	conn := registryConnection{
		ref:            repoRef,
		client:         &auth.Client{Client: http.DefaultClient},
		plainHTTPKnown: true,
		plainHTTP:      false,
	}
	var buf bytes.Buffer
	err = listRegistryTags(ctx, &buf, conn, false, false, false, false)
	require.Error(t, err)
}
