// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestImageArchivePush(t *testing.T) {
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

	_, _, err = e2e.Zarf(t, "package", "deploy", path, "--confirm", "--skip-version-check")
	require.NoError(t, err)

	_, _, err = e2e.Zarf(t, "package", "remove", "image-tar", "--confirm", "--skip-version-check")
	require.NoError(t, err)
}

// FIXME: probably can tie this into a different e2e test, or create a new find-images test for archives near the othersx
func TestImageArchiveV1Beta1(t *testing.T) {
	fixture := filepath.Join("src", "pkg", "packager", "testdata", "find-images", "image-archives")
	source := t.TempDir()
	require.NoError(t, os.CopyFS(source, os.DirFS(fixture)))
	definition := `apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: archive-ids
components:
  - name: app
    manifests:
      - name: scratch
        namespace: default
        files:
          - deployment.yaml
    imageArchives:
      - path: scratch.tar
        images:
          - name: scratch:latest
            id: app
          - name: scratch:stale
`
	definitionPath := filepath.Join(source, "zarf.yaml")
	require.NoError(t, os.WriteFile(definitionPath, []byte(definition), 0o600))
	stdout, stderr, err := e2e.Zarf(t, "dev", "find-images", source, "--skip-cosign", "--update", "--features=v1beta1=true")
	require.NoError(t, err, "%s\n%s", stdout, stderr)
	var found v1beta1.Package
	require.NoError(t, yaml.Unmarshal([]byte(stdout), &found))
	want := []v1beta1.ArchiveImage{{Name: "docker.io/library/scratch:latest", ID: "app"}}
	require.Equal(t, want, found.Components[0].ImageArchives[0].Images)
	updated, err := os.ReadFile(definitionPath)
	require.NoError(t, err)
	var authored v1beta1.Package
	require.NoError(t, yaml.Unmarshal(updated, &authored))
	require.Equal(t, want, authored.Components[0].ImageArchives[0].Images)

	output := t.TempDir()
	stdout, stderr, err = e2e.Zarf(t, "package", "create", source, "-o", output, "--skip-sbom", "--confirm", "--features=v1beta1=true")
	require.NoError(t, err, "%s\n%s", stdout, stderr)
	packagePath := filepath.Join(output, fmt.Sprintf("zarf-package-archive-ids-%s.tar.zst", e2e.Arch))
	pkg, err := layout.LoadFromTar(t.Context(), packagePath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkg.Cleanup()) })
	b, err := os.ReadFile(filepath.Join(pkg.DirPath(), layout.ZarfYAML))
	require.NoError(t, err)
	beta, err := pkgcfg.ParseAs(t.Context(), b, pkgcfg.V1Beta1)
	require.NoError(t, err)
	image, err := beta.Components[0].GetImageByID("app")
	require.NoError(t, err)
	require.Equal(t, "docker.io/library/scratch:latest", image.Reference)
	alpha, err := pkgcfg.ParseAs(t.Context(), b, pkgcfg.V1Alpha1)
	require.NoError(t, err)
	require.Equal(t, []string{image.Reference}, alpha.Components[0].ImageArchives[0].Images)
}
