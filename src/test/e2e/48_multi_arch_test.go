// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// This digest is the multi-arch index manifest of ghcr.io/stefanprodan/podinfo:6.4.0.
const multiArchPodinfoIndexDigest = "sha256:57a654ace69ec02ba8973093b6a786faa15640575fbf0dbb603db55aca2ccec8"

func TestMultiArchPackage(t *testing.T) {
	t.Log("E2E: multi-arch package create + deploy")

	pkgDefinitionPath := filepath.Join("src", "test", "packages", "48-multi-arch")
	tmpdir := t.TempDir()

	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", pkgDefinitionPath, "-o", tmpdir, "--confirm", "--skip-sbom")
	require.NoError(t, err, stdOut, stdErr)

	pkgPath := filepath.Join(tmpdir, "zarf-package-multi-arch-multi-0.0.1.tar.zst")
	require.FileExists(t, pkgPath, "package filename must include the multi architecture suffix")

	pkgLayout, err := layout.LoadFromTar(t.Context(), pkgPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	idxBytes, err := os.ReadFile(filepath.Join(pkgLayout.GetImageDirPath(), "index.json"))
	require.NoError(t, err)
	var idx ocispec.Index
	require.NoError(t, json.Unmarshal(idxBytes, &idx))

	var rootDigest string
	for _, m := range idx.Manifests {
		if strings.Contains(m.Annotations[ocispec.AnnotationRefName], multiArchPodinfoIndexDigest) {
			require.Equal(t, ocispec.MediaTypeImageIndex, m.MediaType, "multi-arch image must be stored as an OCI index")
			rootDigest = m.Digest.String()
			break
		}
	}
	require.Equal(t, multiArchPodinfoIndexDigest, rootDigest, "expected to find the podinfo index manifest in the package layout")

	blobPath := filepath.Join(pkgLayout.GetImageDirPath(), "blobs", "sha256", strings.TrimPrefix(rootDigest, "sha256:"))
	b, err := os.ReadFile(blobPath)
	require.NoError(t, err)
	var pulledIdx ocispec.Index
	require.NoError(t, json.Unmarshal(b, &pulledIdx))
	require.Greater(t, len(pulledIdx.Manifests), 1, "expected multiple platform manifests under the index")

	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", pkgPath, "--confirm", "--skip-version-check")
	require.NoError(t, err, stdOut, stdErr)
	t.Cleanup(func() {
		_, _, err = e2e.Zarf(t, "package", "remove", "multi-arch", "--confirm", "--skip-version-check")
		require.NoError(t, err)
	})

	c, err := cluster.New(t.Context())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	pod, err := c.Clientset.CoreV1().Pods("multi-arch-test").Get(ctx, "multi-arch-podinfo", metav1.GetOptions{})
	require.NoError(t, err)

	require.Len(t, pod.Spec.Containers, 1)
	require.Contains(t, pod.Spec.Containers[0].Image, "@"+multiArchPodinfoIndexDigest,
		"deployed pod image reference must preserve the index digest so kubelet can pick the right platform")
}
