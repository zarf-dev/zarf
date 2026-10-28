// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestFindImages(t *testing.T) {
	t.Log("E2E: Find Images")

	t.Run("package and component config formats", func(t *testing.T) {
		t.Parallel()
		packagePath := filepath.Join("src", "test", "packages", "13-find-images-formats")
		for _, tc := range []struct {
			name string
			path string
			want string
		}{
			{
				name: "v1alpha1 package",
				path: packagePath,
				want: `components:
  - name: app
    images:
      - docker.io/library/nginx:1.27`,
			},
			{
				name: "v1beta1 package",
				path: filepath.Join(packagePath, "zarf-v1beta1.yaml"),
				want: `components:
  - name: app
    images:
      - name: nginx:1.27
        id: "web"
        source: daemon
      - name: docker.io/library/nginx:1.27
        id: "worker"
        source: daemon`,
			},
			{
				name: "v1beta1 component",
				path: filepath.Join(packagePath, "component.yaml"),
				want: `component:
  images:
    - name: nginx:1.27
      id: "web"
    - name: docker.io/library/nginx:1.27
      id: "worker"`,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				stdOut, stdErr, err := e2e.Zarf(t, "dev", "find-images", tc.path, "--skip-cosign")
				require.NoError(t, err, stdOut, stdErr)
				require.Contains(t, stdOut, tc.want)
				if tc.name == "v1beta1 package" {
					require.NotContains(t, stdOut, "example.com/manual:1")
				}
			})
		}
	})

	t.Run("update package and component config formats", func(t *testing.T) {
		t.Parallel()
		image := "docker.io/library/nginx:1.27"

		t.Run("v1alpha1 package", func(t *testing.T) {
			b := updateFindImagesFixture(t, "zarf.yaml")
			var updated v1alpha1.ZarfPackage
			require.NoError(t, yaml.Unmarshal(b, &updated))
			require.Len(t, updated.Components, 1)
			require.Equal(t, []string{image}, updated.Components[0].Images)
		})

		t.Run("v1beta1 package", func(t *testing.T) {
			b := updateFindImagesFixture(t, "zarf-v1beta1.yaml")
			var updated v1beta1.Package
			require.NoError(t, yaml.Unmarshal(b, &updated))
			require.Len(t, updated.Components, 1)
			require.Equal(t, []v1beta1.Image{
				{Name: "nginx:1.27", ID: "web", Source: "daemon"},
				{Name: image, ID: "worker", Source: "daemon"},
			}, updated.Components[0].Images)
		})

		t.Run("v1beta1 component", func(t *testing.T) {
			b := updateFindImagesFixture(t, "component.yaml")
			var updated v1beta1.ComponentConfig
			require.NoError(t, yaml.Unmarshal(b, &updated))
			require.Equal(t, []v1beta1.Image{
				{Name: "nginx:1.27", ID: "web"},
				{Name: image, ID: "worker"},
			}, updated.Component.Images)
		})
	})

	t.Run("zarf prepare find-images", func(t *testing.T) {
		t.Parallel()
		// Test `zarf prepare find-images` for a remote asset
		stdOut, stdErr, err := e2e.Zarf(t, "prepare", "find-images", "examples/helm-charts")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, "ghcr.io/stefanprodan/podinfo:6.4.0", "The chart image should be found by Zarf")
		// Test `zarf prepare find-images` with a chart that uses helm annotations
		stdOut, stdErr, err = e2e.Zarf(t, "prepare", "find-images", "src/test/packages/00-helm-annotations")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, "registry1.dso.mil/ironbank/opensource/istio/pilot:1.17.2", "The pilot image should be found by Zarf")
	})

	t.Run("zarf prepare find-images --kube-version", func(t *testing.T) {
		t.Parallel()
		controllerImageWithTag := "quay.io/jetstack/cert-manager-controller:v1.11.1"
		controlImageWithSignature := "quay.io/jetstack/cert-manager-controller:sha256-4f1782c8316f34aae6b9ab823c3e6b7e6e4d92ec5dac21de6a17c3da44c364f1.sig"

		// Test `zarf prepare find-images` on a chart that has a `kubeVersion` declaration greater than the Helm default (v1.20.0)
		// This should pass because we build Zarf specifying the kubeVersion value from the kubernetes client-go library instead
		stdOut, stdErr, err := e2e.Zarf(t, "prepare", "find-images", "src/test/packages/00-kube-version-override")
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdOut, controllerImageWithTag, "The chart image should be found by Zarf")
		require.Contains(t, stdOut, controlImageWithSignature, "The image signature should be found by Zarf")

		// Test `zarf prepare find-images` with `--kube-version` specified and less than than the declared minimum (v1.21.0)
		stdOut, stdErr, err = e2e.Zarf(t, "prepare", "find-images", "--kube-version=v1.20.0", "src/test/packages/00-kube-version-override")
		require.Error(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "could not render the Helm template for chart cert-manager", "The kubeVersion declaration should prevent this from templating")
	})

	t.Run("zarf dev find-images with helm or manifest vars", func(t *testing.T) {
		t.Parallel()
		registry := "zarf.dev"
		agentTag := "test"

		stdOut, _, err := e2e.Zarf(t, "dev", "find-images", ".", "--registry-url", registry, "--create-set", fmt.Sprintf("agent_image_tag=%s", agentTag), "--skip-cosign")

		require.NoError(t, err)
		internalRegistryImage := fmt.Sprintf("%s/%s:%s", registry, "zarf-dev/zarf/agent", agentTag)
		require.Contains(t, stdOut, internalRegistryImage)
		// busybox image is in git init package
		require.Contains(t, stdOut, "busybox:latest")

		path := filepath.Join("src", "test", "packages", "13-find-images-with-vars")
		stdOut, _, err = e2e.Zarf(t, "dev", "find-images", path, "--deploy-set", "PODINFO_IMAGE=zarf.dev/podinfo:latest", "--skip-cosign")
		require.NoError(t, err)
		require.Contains(t, stdOut, "ghcr.io/zarf-dev/zarf/agent:v0.36.1")
		require.Contains(t, stdOut, "zarf.dev/podinfo:latest")
	})

	t.Run("zarf test find images --why w/ helm chart success", func(t *testing.T) {
		t.Parallel()
		testPackagePath := filepath.Join("examples", "wordpress")
		sets := []string{"WORDPRESS_USERNAME=zarf", "WORDPRESS_PASSWORD=fake", "WORDPRESS_EMAIL=hello@defenseunicorns.com", "WORDPRESS_FIRST_NAME=zarf", "WORDPRESS_LAST_NAME=zarf", "WORDPRESS_BLOG_NAME=blog"}
		deploysSet := strings.Join(sets, ",")
		stdOut, _, err := e2e.Zarf(t, "dev", "find-images", testPackagePath, "--why", "docker.io/bitnamilegacy/apache-exporter:1.0.10-debian-12-r55", "--deploy-set", deploysSet)
		require.NoError(t, err)
		require.Contains(t, stdOut, "component: wordpress")
		require.Contains(t, stdOut, "chart: wordpress")
		require.Contains(t, stdOut, "image: docker.io/bitnamilegacy/wordpress:6.8.2-debian-12-r4")
	})

	t.Run("zarf test find images --why w/ manifests success", func(t *testing.T) {
		t.Parallel()
		testPackagePath := filepath.Join("examples", "manifests")

		stdOut, _, err := e2e.Zarf(t, "dev", "find-images", testPackagePath, "--why", "httpd:alpine3.18")
		require.NoError(t, err)
		require.Contains(t, stdOut, "component: httpd-local")
		require.Contains(t, stdOut, "manifest: simple-httpd-deployment")
		require.Contains(t, stdOut, "image: httpd:alpine3.18")
	})
}

func updateFindImagesFixture(t *testing.T, filename string) []byte {
	t.Helper()
	fixturePath := filepath.Join("src", "test", "packages", "13-find-images-formats")
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(fixturePath)))
	path := filepath.Join(dir, filename)
	stdOut, stdErr, err := e2e.Zarf(t, "dev", "find-images", path, "--update", "--skip-cosign")
	require.NoError(t, err, stdOut, stdErr)
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return b
}

func TestFindImagesArchivesV1Beta1(t *testing.T) {
	t.Parallel()

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
          - name: docker.io/library/scratch:latest
            id: worker
          - name: scratch:stale
`
	definitionPath := filepath.Join(source, "zarf.yaml")
	require.NoError(t, os.WriteFile(definitionPath, []byte(definition), 0o600))
	stdout, stderr, err := e2e.Zarf(t, "dev", "find-images", source, "--skip-cosign", "--update", "--features=v1beta1=true")
	require.NoError(t, err, "%s\n%s", stdout, stderr)
	var found v1beta1.Package
	require.NoError(t, yaml.Unmarshal([]byte(stdout), &found))
	want := []v1beta1.ArchiveImage{
		{Name: "docker.io/library/scratch:latest", ID: "app"},
		{Name: "docker.io/library/scratch:latest", ID: "worker"},
	}
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
	for _, id := range []string{"app", "worker"} {
		image, err := beta.Components[0].GetImageByID(id)
		require.NoError(t, err)
		require.Equal(t, "docker.io/library/scratch:latest", image.Reference)
	}
	alpha, err := pkgcfg.ParseAs(t.Context(), b, pkgcfg.V1Alpha1)
	require.NoError(t, err)
	require.Equal(t, []string{"docker.io/library/scratch:latest", "docker.io/library/scratch:latest"}, alpha.Components[0].ImageArchives[0].Images)
}
