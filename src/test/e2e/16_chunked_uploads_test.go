// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"oras.land/oras-go/v2/content"
)

func TestChunkedUploads(t *testing.T) {
	t.Log("E2E: Publish, copy, and mirror large layers through a registry request-size limit")
	ctx := testutil.TestContext(t)
	upstream := testutil.SetupInMemoryRegistryDynamic(ctx, t)
	upstreamURL, err := url.Parse("http://" + upstream)
	require.NoError(t, err)
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	var patches atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/blobs/uploads/") && (r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			if r.ContentLength > oci.PushChunkSize {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
			if r.Method == http.MethodPatch {
				patches.Add(1)
			}
		}
		proxy.ServeHTTP(w, r)
	}))
	defer server.Close()
	address := strings.TrimPrefix(server.URL, "http://")

	// An incompressible layer also keeps the package's image blob above the limit.
	data := testutil.RandomBytes(t, int(oci.PushChunkSize)+1024)
	layer := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, data)
	control := testutil.NewRepo(t, address+"/control")
	err = control.Push(ctx, layer, bytes.NewReader(data))
	require.ErrorContains(t, err, "413", "the monolithic control must exceed the proxy limit")

	imageRef := upstream + "/fixtures/chunked:v1"
	source := testutil.NewRepo(t, imageRef)
	require.NoError(t, source.Push(ctx, layer, bytes.NewReader(data)))
	config := testutil.PushBlob(ctx, t, source, ocispec.MediaTypeImageConfig, []byte(fmt.Sprintf(`{"architecture":%q,"os":"linux"}`, e2e.Arch)))
	manifest := testutil.PushManifest(ctx, t, source, config, []ocispec.Descriptor{layer})
	require.NoError(t, source.Tag(ctx, manifest, "v1"))

	packageDir := t.TempDir()
	definition := fmt.Sprintf("kind: ZarfPackageConfig\nmetadata:\n  name: chunked-uploads\n  version: v1\ncomponents:\n  - name: image\n    required: true\n    images:\n      - %s\n", imageRef)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "zarf.yaml"), []byte(definition), 0o600))
	outputDir := t.TempDir()
	stdout, stderr, err := e2e.Zarf(t, "package", "create", packageDir, "-o", outputDir, "--plain-http", "--skip-sbom", "--confirm")
	require.NoError(t, err, stdout, stderr)
	packagePath := filepath.Join(outputDir, fmt.Sprintf("zarf-package-chunked-uploads-%s-v1.tar.zst", e2e.Arch))
	require.FileExists(t, packagePath)

	for _, tc := range []struct {
		name string
		args []string
		repo string
	}{
		{name: "publish", args: []string{"package", "publish", packagePath, "oci://" + address + "/published", "--plain-http", "--confirm"}, repo: address + "/published/chunked-uploads"},
		{name: "copy", args: []string{"package", "publish", "oci://" + address + "/published/chunked-uploads:v1", "oci://" + address + "/copied", "--plain-http", "--confirm"}, repo: address + "/copied/chunked-uploads"},
		{name: "mirror", args: []string{"package", "mirror-resources", packagePath, "--registry-url", address + "/mirrored", "--images", "--no-img-checksum", "--plain-http", "--confirm"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := patches.Load()
			stdout, stderr, err := e2e.Zarf(t, tc.args...)
			require.NoError(t, err, stdout, stderr)
			require.Greater(t, patches.Load(), before, "the large layer must use PATCH uploads")
			repository := tc.repo
			if tc.name == "mirror" {
				ref, err := transform.ImageTransformHostWithoutChecksum(address+"/mirrored", imageRef)
				require.NoError(t, err)
				repository = ref
			}
			destination := testutil.NewRepo(t, repository)
			reader, err := destination.Fetch(ctx, layer)
			require.NoError(t, err)
			fetched, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			require.Equal(t, data, fetched, "the registry must retain the original blob and digest")
		})
	}
	t.Run("component publish", func(t *testing.T) {
		componentDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(componentDir, "large.bin"), data, 0o600))
		definition := `apiVersion: zarf.dev/v1beta1
kind: ZarfComponentConfig
metadata:
  name: chunked-component
  version: v1
component:
  files:
    - source: large.bin
      destination: /tmp/large.bin
`
		componentPath := filepath.Join(componentDir, "component.yaml")
		require.NoError(t, os.WriteFile(componentPath, []byte(definition), 0o600))
		before := patches.Load()
		stdout, stderr, err := e2e.Zarf(t, "component", "publish", componentPath, "oci://"+address+"/components", "--plain-http", "--features=v1beta1=true")
		require.NoError(t, err, stdout, stderr)
		require.Greater(t, patches.Load(), before, "large component files must use PATCH uploads")

		destination := testutil.NewRepo(t, address+"/components/chunked-component:v1")
		manifestDesc, reader, err := destination.FetchReference(ctx, "v1")
		require.NoError(t, err)
		manifestData, err := content.ReadAll(reader, manifestDesc)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		var published ocispec.Manifest
		require.NoError(t, json.Unmarshal(manifestData, &published))
		var largeLayers int
		for _, layer := range published.Layers {
			if layer.Size <= oci.PushChunkSize {
				continue
			}
			largeLayers++
			reader, err := destination.Fetch(ctx, layer)
			require.NoError(t, err)
			// Verify the uploaded archive against its declared size and digest.
			_, err = content.ReadAll(reader, layer)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
		}
		require.Positive(t, largeLayers)
	})
}
