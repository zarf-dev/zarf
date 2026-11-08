// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	goyaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/test/testutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

func TestComposabilityExample(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	packageDir := t.TempDir()
	require.NoError(t, os.CopyFS(packageDir, os.DirFS(filepath.Join("examples", "composable-packages"))))
	componentPath := filepath.Join(packageDir, "components", "web-server.yaml")
	registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	privateKey := filepath.Join("src", "test", "packages", "zarf-test.prv-key")
	stdOut, stdErr, err := e2e.Zarf(t, "component", "publish", componentPath, "oci://"+registryURL, "--plain-http", "--signing-key", privateKey)
	require.NoError(t, err, stdOut, stdErr)

	definitionPath := filepath.Join(packageDir, "zarf.yaml")
	definition, err := os.ReadFile(definitionPath)
	require.NoError(t, err)
	var pkgDefinition v1beta1.Package
	require.NoError(t, goyaml.Unmarshal(definition, &pkgDefinition))
	pkgDefinition.Components[1].Import.Remote[0].URL = "oci://" + registryURL + "/web-server:1.0.0"
	definition, err = goyaml.Marshal(pkgDefinition)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(definitionPath, definition, 0o600))

	outputDir := t.TempDir()
	stdOut, stdErr, err = e2e.Zarf(t, "package", "create", packageDir, "-o", outputDir, "--plain-http", "--confirm", "--cache", packageDir, "--features=v1beta1=true,values=true")
	require.NoError(t, err, stdOut, stdErr)
	tarPath := filepath.Join(outputDir, fmt.Sprintf("zarf-package-composable-packages-%s.tar.zst", e2e.Arch))
	pkgLayout, err := layout.LoadFromTar(t.Context(), tarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	for _, tt := range []struct {
		name, namespace, message string
		index                    int
	}{
		{"local", "imports-local", "Hello from the local component import!", 0},
		{"remote", "imports-remote", "Hello from the published component import!", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			definitionPkg := pkgLayout.Definition()
			require.Len(t, definitionPkg.Components, 2)
			component := definitionPkg.Components[tt.index]
			require.Len(t, component.Images, 1)
			require.Len(t, component.Charts, 1)
			require.Equal(t, tt.namespace, component.Charts[0].Namespace)

			for _, configuration := range []struct{ name, connectName, message string }{
				{"defaults", "web-server-" + tt.name, tt.message},
				{"overrides", "custom-" + tt.name, "Custom " + tt.name + " greeting"},
			} {
				t.Run(configuration.name, func(t *testing.T) {
					args := []string{"package", "inspect", "manifests", tarPath, "--components", component.Name, "--features=v1beta1=true,values=true"}
					if configuration.name == "overrides" {
						args = append(args, "--set-values", fmt.Sprintf("webServers.%s.connectName=%s,webServers.%s.message=%s", tt.name, configuration.connectName, tt.name, configuration.message))
					}
					output, stdErr, err := e2e.Zarf(t, args...)
					require.NoError(t, err, output, stdErr)
					decoder := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(output), 4096)
					resources := map[string]bool{}
					for {
						var data json.RawMessage
						err := decoder.Decode(&data)
						if errors.Is(err, io.EOF) {
							break
						}
						require.NoError(t, err)
						if len(data) == 0 || string(data) == "null" {
							continue
						}
						var header struct {
							Kind     string
							Metadata struct{ Name string }
						}
						require.NoError(t, json.Unmarshal(data, &header))
						key := header.Kind + "/" + header.Metadata.Name
						require.False(t, resources[key], "duplicate resource %s", key)
						resources[key] = true
						switch header.Kind {
						case "Deployment":
							var deployment appsv1.Deployment
							require.NoError(t, json.Unmarshal(data, &deployment))
							require.Equal(t, component.Images[0].Name, deployment.Spec.Template.Spec.Containers[0].Image)
							require.Equal(t, "web-content", deployment.Spec.Template.Spec.Volumes[0].ConfigMap.Name)
						case "Service":
							var service corev1.Service
							require.NoError(t, json.Unmarshal(data, &service))
							require.Equal(t, configuration.connectName, service.Labels["zarf.dev/connect-name"])
							require.Equal(t, "web-server", service.Spec.Selector["app"])
						case "ConfigMap":
							var configMap corev1.ConfigMap
							require.NoError(t, json.Unmarshal(data, &configMap))
							require.Contains(t, configMap.Data["index.html"], "<h1>"+configuration.message+"</h1>")
						default:
							t.Fatalf("unexpected resource kind %s", header.Kind)
						}
					}
					require.Len(t, resources, 3)
				})
			}
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
