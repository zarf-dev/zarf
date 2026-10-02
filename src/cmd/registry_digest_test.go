// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
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
			err := resolveDigest(ctx, &buf, conn, false, false, tt.fullRef, "")
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
	err := resolveDigest(ctx, &buf, conn, false, false, false, "")
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

	conn := registryConnection{
		ref:            imageRef,
		client:         &auth.Client{Client: http.DefaultClient},
		plainHTTPKnown: true,
		plainHTTP:      false,
	}
	var buf bytes.Buffer
	err = resolveDigest(ctx, &buf, conn, false, false, false, "")
	require.Error(t, err)
}

func TestResolveDigestPlatform(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	address := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	repo := testutil.NewRepo(t, address+"/multi-arch")

	amd64 := testutil.PushSinglePlatformImage(ctx, t, repo, "amd64")
	amd64.Platform = &ocispec.Platform{OS: "linux", Architecture: "amd64"}
	arm64 := testutil.PushSinglePlatformImage(ctx, t, repo, "arm64")
	arm64.Platform = &ocispec.Platform{OS: "linux", Architecture: "arm64"}
	index := testutil.PushIndex(ctx, t, repo, []ocispec.Descriptor{amd64, arm64})
	require.NoError(t, repo.Tag(ctx, index, "multi"))

	imageRef := fmt.Sprintf("%s/multi-arch:multi", address)
	conn := registryConnection{ref: imageRef, client: &auth.Client{Client: http.DefaultClient}}

	tests := []struct {
		name     string
		platform string
		want     string
		wantErr  bool
	}{
		{name: "no platform returns the index digest", want: index.Digest.String()},
		{name: "all returns the index digest", platform: "all", want: index.Digest.String()},
		{name: "platform selects the matching manifest", platform: "linux/amd64", want: amd64.Digest.String()},
		{name: "unmatched platform errors", platform: "linux/riscv64", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := resolveDigest(ctx, &buf, conn, false, false, false, tt.platform)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, strings.TrimSpace(buf.String()))
		})
	}
}

func TestParseTargetPlatformInvalid(t *testing.T) {
	t.Parallel()
	_, err := parseTargetPlatform("linux")
	require.Error(t, err)
}

func writeOCILayoutTar(t *testing.T, dir, tarPath string) {
	t.Helper()
	f, err := os.Create(tarPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	tw := tar.NewWriter(f)
	defer func() { require.NoError(t, tw.Close()) }()

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	require.NoError(t, err)
}

func newOCILayoutTar(ctx context.Context, t *testing.T, manifests func(store *oci.Store)) string {
	t.Helper()
	layoutDir := t.TempDir()
	store, err := oci.NewWithContext(ctx, layoutDir)
	require.NoError(t, err)
	manifests(store)

	tarPath := filepath.Join(t.TempDir(), "image.tar")
	writeOCILayoutTar(t, layoutDir, tarPath)
	return tarPath
}

func TestTarballDigest(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	var want ocispec.Descriptor
	tarPath := newOCILayoutTar(ctx, t, func(store *oci.Store) {
		desc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "application/vnd.test.artifact", oras.PackManifestOptions{})
		require.NoError(t, err)
		require.NoError(t, store.Tag(ctx, desc, "1.0.0"))
		want = desc
	})

	var buf bytes.Buffer
	require.NoError(t, tarballDigest(ctx, &buf, tarPath, "1.0.0"))
	require.Equal(t, want.Digest.String(), strings.TrimSpace(buf.String()))
}

func TestTarballDigestNoRefSingleManifest(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	var want ocispec.Descriptor
	tarPath := newOCILayoutTar(ctx, t, func(store *oci.Store) {
		desc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "application/vnd.test.artifact", oras.PackManifestOptions{})
		require.NoError(t, err)
		want = desc
	})

	var buf bytes.Buffer
	require.NoError(t, tarballDigest(ctx, &buf, tarPath, ""))
	require.Equal(t, want.Digest.String(), strings.TrimSpace(buf.String()))
}

func TestTarballDigestNoRefMultipleManifestsErrors(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	tarPath := newOCILayoutTar(ctx, t, func(store *oci.Store) {
		_, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "application/vnd.test.artifact.one", oras.PackManifestOptions{})
		require.NoError(t, err)
		_, err = oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "application/vnd.test.artifact.two", oras.PackManifestOptions{})
		require.NoError(t, err)
	})

	var buf bytes.Buffer
	err := tarballDigest(ctx, &buf, tarPath, "")
	require.Error(t, err)
}

func TestTarballDigestUnknownRef(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	tarPath := newOCILayoutTar(ctx, t, func(store *oci.Store) {
		_, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "application/vnd.test.artifact", oras.PackManifestOptions{})
		require.NoError(t, err)
	})

	var buf bytes.Buffer
	err := tarballDigest(ctx, &buf, tarPath, "does-not-exist")
	require.Error(t, err)
}

func TestTarballDigestInvalidPath(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	var buf bytes.Buffer
	err := tarballDigest(ctx, &buf, filepath.Join(t.TempDir(), "does-not-exist.tar"), "")
	require.Error(t, err)
}
