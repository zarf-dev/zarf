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

	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
)

func TestComposabilityExample(t *testing.T) {
	t.Parallel()

	// Skip for Windows until path separators in packages are standardized.
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tmpDir := t.TempDir()
	composeExample := filepath.Join("examples", "composable-packages")
	_, _, err := e2e.Zarf(t, "package", "create", composeExample, "-o", tmpDir, "--no-color", "--confirm", "--zarf-cache", tmpDir)
	require.NoError(t, err)

	tarPath := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-composable-packages-%s.tar.zst", e2e.Arch))
	pkgLayout, err := layout2.LoadFromTar(context.Background(), tarPath, layout2.PackageLayoutOptions{})
	require.NoError(t, err)

	require.Len(t, pkgLayout.Pkg.Components, 2)
	b, err := goyaml.Marshal(pkgLayout.Pkg.Components)
	require.NoError(t, err)

	expectedYaml := fmt.Sprintf(`- name: local-games-path
  description: Example of a local composed package with a unique description for this component
  required: true
  manifests:
  - name: multi-games
    namespace: dos-games
    files:
    - ../dos-games/manifests/deployment.yaml
    - ../dos-games/manifests/service.yaml
    - quake-service.yaml
  images:
  - ghcr.io/zarf-dev/doom-game:0.0.1
- name: oci-games-url
  manifests:
  - name: multi-games
    namespace: dos-games
    files:
    - ../../../../../../..%s/oci/dirs/9ece174e362633b637e3c6b554b70f7d009d0a27107bee822336fdf2ce9a9def/manifests/multi-games-0.yaml
    - ../../../../../../..%s/oci/dirs/9ece174e362633b637e3c6b554b70f7d009d0a27107bee822336fdf2ce9a9def/manifests/multi-games-1.yaml
  images:
  - ghcr.io/zarf-dev/doom-game:0.0.1
  actions:
    onDeploy:
      before:
      - cmd: ./zarf tools kubectl get -n dos-games deployment -o jsonpath={.items[0].metadata.creationTimestamp}
      after:
      - wait:
          cluster:
            kind: deployment
            name: game
            namespace: dos-games
            condition: available
`, tmpDir, tmpDir)
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
	pkgLayout, err := layout2.LoadFromTar(context.Background(), tarPath, layout2.PackageLayoutOptions{})
	require.NoError(t, err)

	require.Len(t, pkgLayout.Pkg.Components, 1)
	b, err := goyaml.Marshal(pkgLayout.Pkg.Components)
	require.NoError(t, err)

	expectedYaml := `- name: test-compose-package
  description: A contrived example for podinfo using many Zarf primitives for compose testing
  required: true
  only:
    localOS: linux
  manifests:
  - name: connect-service
    namespace: podinfo-override
    files:
    - files/service.yaml
    - files/service.yaml
    kustomizations:
    - files
    - files
  - name: connect-service-two
    namespace: podinfo-compose-two
    files:
    - files/service.yaml
    kustomizations:
    - files
  charts:
  - name: podinfo-compose
    version: 6.4.0
    url: oci://ghcr.io/stefanprodan/charts/podinfo
    namespace: podinfo-override
    releaseName: podinfo-override
    valuesFiles:
    - files/test-values.yaml
    - files/test-values.yaml
  - name: podinfo-compose-two
    version: 6.4.0
    url: oci://ghcr.io/stefanprodan/charts/podinfo
    namespace: podinfo-compose-two
    releaseName: podinfo-compose-two
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
  - source: files/coffee-ipsum.txt
    target: coffee-ipsum.txt
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
      - dir: .
        cmd: ls
    onDeploy:
      after:
      - cmd: cat coffee-ipsum.txt
      - wait:
          cluster:
            kind: deployment
            name: podinfo-compose-two
            namespace: podinfo-compose-two
            condition: available
`
	require.YAMLEq(t, expectedYaml, string(b))
}

func TestComposabilityBadLocalOS(t *testing.T) {
	t.Parallel()

	composeTestBadLocalOS := filepath.Join("src", "test", "packages", "09-composable-packages", "bad-local-os")
	_, stdErr, err := e2e.Zarf(t, "package", "create", composeTestBadLocalOS, "-o", "build", "--no-color", "--confirm")
	require.Error(t, err)
	require.Contains(t, stdErr, "\"only.localOS\" \"linux\" cannot be redefined as \"windows\" during compose")
}
