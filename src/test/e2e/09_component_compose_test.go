// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestComposabilityExample(t *testing.T) {
	t.Parallel()

	// Skip for Windows until path separators in packages are standardized.
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tmpDir := t.TempDir()
	composeTest := filepath.Join("src", "test", "packages", "09-composable-v1beta1")
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", composeTest, "-o", tmpDir, "--no-color", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	tarPath := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-test-compose-v1beta1-%s.tar.zst", e2e.Arch))
	pkgLayout, err := layout.LoadFromTar(t.Context(), tarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	components := convert.PackageToV1beta1(pkgLayout.Definition()).Components
	require.Len(t, components, 2)
	b, err := goyaml.Marshal(components)
	require.NoError(t, err)

	expectedYaml := `- name: local-import
  description: Import and customize a component
  manifests:
  - name: config
    namespace: overridden
    files:
    - components/config.yaml
    - extra.yaml
- name: second-import
  manifests:
  - name: config
    namespace: original
    files:
    - components/config.yaml
`
	require.YAMLEq(t, expectedYaml, string(b))
}

func TestFullComposability(t *testing.T) {
	t.Parallel()

	// Skip for Windows until path separators in packages are standardized.
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tmpDir := t.TempDir()
	composeTest := filepath.Join("src", "test", "packages", "09-composable-packages")
	_, _, err := e2e.Zarf(t, "package", "create", composeTest, "-o", tmpDir, "--no-color", "--confirm")
	require.NoError(t, err)

	tarPath := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-test-compose-package-%s-0.0.1.tar.zst", e2e.Arch))
	pkgLayout, err := layout.LoadFromTar(context.Background(), tarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)

	require.Len(t, pkgLayout.Definition().Components, 1)
	b, err := goyaml.Marshal(convert.PackageToV1alpha1(pkgLayout.Definition()).Components)
	require.NoError(t, err)

	expectedYaml := `- name: test-compose-package
  description: A contrived example for podinfo using many Zarf primitives for compose testing
  required: true
  only:
    localOS: linux
  manifests:
  - name: connect-service
    namespace: podinfo-override
    template: false
    files:
    - files/service.yaml
    - files/service.yaml
    kustomizations:
    - files
    - files/
  - name: connect-service-two
    namespace: podinfo-compose-two
    template: false
    files:
    - files/service.yaml
    kustomizations:
    - files/
  charts:
  - name: podinfo-compose
    version: 6.4.0
    url: oci://ghcr.io/stefanprodan/charts/podinfo
    namespace: podinfo-override
    releaseName: podinfo-override
    schemaValidation: true
    valuesFiles:
    - files/test-values.yaml
    - files/test-values.yaml
  - name: podinfo-compose-two
    version: 6.4.0
    url: oci://ghcr.io/stefanprodan/charts/podinfo
    namespace: podinfo-compose-two
    releaseName: podinfo-compose-two
    schemaValidation: true
    valuesFiles:
    - files/test-values.yaml
  dataInjections:
  - source: files
    target:
      namespace: podinfo-compose
      selector: app.kubernetes.io/name=podinfo-compose
      container: podinfo
      path: /home/app/service.yaml
  - source: files
    target:
      namespace: podinfo-compose
      selector: app.kubernetes.io/name=podinfo-compose
      container: podinfo
      path: /home/app/service.yaml
  files:
  - source: files/coffee-ipsum.txt
    target: coffee-ipsum.txt
    template: false
  - source: files/coffee-ipsum.txt
    target: coffee-ipsum.txt
    template: false
  images:
  - ghcr.io/stefanprodan/podinfo:6.4.0
  - ghcr.io/stefanprodan/podinfo:6.4.1
  repos:
  - https://github.com/zarf-dev/zarf-public-test.git
  - https://github.com/zarf-dev/zarf-public-test.git@refs/heads/dragons
  actions:
    onCreate:
      before:
      - dir: sub-package
        cmd: ls
        template: false
      - cmd: ls
        template: false
    onDeploy:
      after:
      - cmd: cat coffee-ipsum.txt
        template: false
      - wait:
          cluster:
            kind: deployment
            name: podinfo-compose-two
            namespace: podinfo-compose-two
            condition: available
        template: false
`
	require.YAMLEq(t, expectedYaml, string(b))
}

func TestComposabilityBadLocalOS(t *testing.T) {
	t.Parallel()

	tmpdir := t.TempDir()
	composeTestBadLocalOS := filepath.Join("src", "test", "packages", "09-composable-packages", "bad-local-os")
	_, stdErr, err := e2e.Zarf(t, "package", "create", composeTestBadLocalOS, "-o", tmpdir, "--no-color", "--confirm")
	require.Error(t, err)
	require.Contains(t, stdErr, "\"only.localOS\" \"linux\" cannot be redefined as \"windows\" during compose")
}
