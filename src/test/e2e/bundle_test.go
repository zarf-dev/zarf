// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func publish(t *testing.T, path string, reg string) {
	cmd := strings.Split(fmt.Sprintf("package publish %s oci://%s --insecure --oci-concurrency=10", path, reg), " ")
	_, _, err := e2e.Zarf(cmd...)
	require.NoError(t, err)
}

var cliver string

func TestBundle(t *testing.T) {
	e2e.SetupDockerRegistry(t, 888)
	defer e2e.TeardownRegistry(t, 888)
	e2e.SetupDockerRegistry(t, 889)
	defer e2e.TeardownRegistry(t, 889)

	cliver = e2e.GetZarfVersion(t)

	pkg := fmt.Sprintf("build/zarf-init-%s-%s.tar.zst", e2e.Arch, cliver)
	publish(t, pkg, "localhost:888")

	pkg = fmt.Sprintf("build/zarf-package-manifests-%s-0.0.1.tar.zst", e2e.Arch)
	publish(t, pkg, "localhost:889")

	testCreate(t)

	testInspect(t)

	testPull(t)
}

func testCreate(t *testing.T) {
	dir := "src/test/packages/60-bundle"
	cmd := strings.Split(fmt.Sprintf("bundle create %s -o oci://%s --set INIT_VERSION=%s --confirm --insecure -l=debug", dir, "localhost:888", cliver), " ")
	_, _, err := e2e.Zarf(cmd...)
	require.NoError(t, err)
}

func testInspect(t *testing.T) {
	ref := fmt.Sprintf("localhost:888/bundle:0.0.1-%s", e2e.Arch)
	cmd := strings.Split(fmt.Sprintf("bundle inspect oci://%s --insecure -l=debug", ref), " ")
	_, _, err := e2e.Zarf(cmd...)
	require.NoError(t, err)
}

func shasMatch(t *testing.T, path string, expected string) {
	actual, err := utils.GetSHA256OfFile(path)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func testPull(t *testing.T) {
	ref := fmt.Sprintf("localhost:888/bundle:0.0.1-%s", e2e.Arch)
	cmd := strings.Split(fmt.Sprintf("bundle pull oci://%s -o build --insecure --oci-concurrency=10 -l=debug", ref), " ")
	_, _, err := e2e.Zarf(cmd...)
	require.NoError(t, err)

	decompressed := "build/decompress-bundle"
	defer e2e.CleanFiles(decompressed)

	cmd = []string{"tools", "archiver", "decompress", fmt.Sprintf("build/zarf-bundle-bundle-%s-0.0.1.tar.zst", e2e.Arch), decompressed}
	_, _, err = e2e.Zarf(cmd...)
	require.NoError(t, err)

	index := ocispec.Index{}
	b, err := os.ReadFile(filepath.Join(decompressed, "index.json"))
	require.NoError(t, err)
	err = json.Unmarshal(b, &index)
	require.NoError(t, err)

	require.Equal(t, 2, len(index.Manifests))

	// for _, desc := range index.Manifests {
	// 	sha := desc.Digest.Encoded()
	// 	shasMatch(t, filepath.Join(blobsDir, sha), desc.Digest.Encoded())

	// 	manifest := ocispec.Manifest{}
	// 	b, err := os.ReadFile(filepath.Join(blobsDir, sha))
	// 	require.NoError(t, err)
	// 	err = json.Unmarshal(b, &manifest)
	// 	require.NoError(t, err)

	// 	require.FileExists(t, filepath.Join(blobsDir, manifest.Config.Digest.Encoded()))

	// 	for _, layer := range manifest.Layers {
	// 		sha := layer.Digest.Encoded()
	// 		require.FileExists(t, filepath.Join(blobsDir, sha))
	// 		shasMatch(t, filepath.Join(blobsDir, sha), layer.Digest.Encoded())
	// 	}
	// }
}
