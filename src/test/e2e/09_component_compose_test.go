// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestComposabilityExample(t *testing.T) {
	t.Parallel()

	// Skip for Windows until path separators in packages are standardized.
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tmpDir := t.TempDir()
	composeExample := filepath.Join("examples", "composable-packages")
	// Copy the complete example so both imports use only its own component files.
	packageDir := t.TempDir()
	require.NoError(t, os.CopyFS(packageDir, os.DirFS(composeExample)))
	componentPath := filepath.Join(packageDir, "components", "application.yaml")

	registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	privateKey := filepath.Join("src", "test", "packages", "zarf-test.prv-key")
	stdOut, stdErr, err := e2e.Zarf(t, "component", "publish", componentPath, "oci://"+registryURL, "--plain-http", "--signing-key", privateKey)
	require.NoError(t, err, stdOut, stdErr)

	definitionPath := filepath.Join(packageDir, "zarf.yaml")
	definition, err := os.ReadFile(definitionPath)
	require.NoError(t, err)
	definition = []byte(strings.ReplaceAll(string(definition), "localhost:5000", registryURL))
	require.NoError(t, os.WriteFile(definitionPath, definition, 0o600))

	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", packageDir, "-o", tmpDir, "--plain-http", "--confirm", "--cache", tmpDir)
	require.NoError(t, err, stdOut, stdErr)

	tarPath := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-composable-packages-%s.tar.zst", e2e.Arch))
	pkgLayout, err := layout.LoadFromTar(context.Background(), tarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	definitionPkg := pkgLayout.Definition()
	require.Equal(t, "zarf.dev/v1beta1", definitionPkg.APIVersion)
	require.Len(t, definitionPkg.Components, 2)
	for _, component := range definitionPkg.Components {
		t.Run(component.Name, func(t *testing.T) {
			expected := map[string]map[string]string{
				"shared-settings":      {"log-level": "info"},
				"application-settings": {"application": "imports-demo"},
			}
			namespace := "imports-remote"
			if component.Name == "local-import" {
				namespace = "imports-local"
				expected["local-settings"] = map[string]string{"message": "Added by the consuming package"}
			} else {
				require.Equal(t, "remote-import", component.Name)
			}
			require.Len(t, component.Manifests, 1)
			require.Equal(t, namespace, component.Manifests[0].Namespace)

			manifestDir, err := pkgLayout.GetComponentDir(t.Context(), t.TempDir(), component.Name, layout.ManifestsComponentDir)
			require.NoError(t, err)
			entries, err := os.ReadDir(manifestDir)
			require.NoError(t, err)
			actual := map[string]map[string]string{}
			for _, entry := range entries {
				data, err := os.ReadFile(filepath.Join(manifestDir, entry.Name()))
				require.NoError(t, err)
				var configMap struct {
					Kind     string
					Metadata struct{ Name string }
					Data     map[string]string
				}
				require.NoError(t, goyaml.Unmarshal(data, &configMap))
				require.Equal(t, "ConfigMap", configMap.Kind)
				require.NotContains(t, actual, configMap.Metadata.Name)
				actual[configMap.Metadata.Name] = configMap.Data
			}
			require.Equal(t, expected, actual)
		})
	}
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
