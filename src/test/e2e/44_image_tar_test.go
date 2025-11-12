// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestImageTarPush(t *testing.T) {
	t.Log("E2E: image tar push")

	pkgDefinitionPath := filepath.Join("src", "test", "packages", "44-image-tar")
	tmpdir := t.TempDir()
	_, _, err := e2e.Zarf(t, "package", "create", pkgDefinitionPath, "-o", tmpdir)
	require.NoError(t, err)

	packageName := fmt.Sprintf("zarf-package-image-tar-%s.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	pkgLayout, err := layout.LoadFromTar(t.Context(), path, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	sbomPath := t.TempDir()
	err = pkgLayout.GetSBOM(t.Context(), sbomPath)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(sbomPath, "ghcr.io_zarf-dev_images_alpine_3.21.3.json"))
	// Verify that the images in image archives were populated properly
	require.Equal(t, []string{"ghcr.io/zarf-dev/images/alpine:3.21.3"}, pkgLayout.Pkg.Components[0].ImageArchives[0].Images)

	_, _, err = e2e.Zarf(t, "package", "mirror-resources", path)
	require.NoError(t, err)
}
