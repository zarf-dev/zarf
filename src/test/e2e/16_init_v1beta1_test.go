// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/test/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestInitV1Beta1(t *testing.T) {
	type chartValues struct {
		Image struct {
			Repository string
			Tag        string
		}
		Persistence  struct{ Size string }
		Secrets      struct{ Htpasswd string }
		NodeSelector map[string]string `yaml:"nodeSelector"`
		Tolerations  []map[string]string
		ReplicaCount int `yaml:"replicaCount"`
	}
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.CopyFS(filepath.Join(root, "packages"), os.DirFS("packages")))
			require.NoError(t, os.CopyFS(filepath.Join(root, ".vex"), os.DirFS(".vex")))
			packageDir := filepath.Join(root, "packages", "v1beta1-init")
			templates, err := filepath.Glob(filepath.Join(packageDir, "*", "*.tpl.yaml"))
			require.NoError(t, err)
			templates = append(templates, filepath.Join(packageDir, "zarf.tpl.yaml"))
			for _, template := range templates {
				stdout, stderr, err := e2e.Zarf(t, "dev", "template", template, "--set-file", filepath.Join(packageDir, "template-values.yaml"),
					"--set", "architecture="+arch+",agent.image=registry.example:5443/team/agent:custom,agent.source=registry")
				require.NoError(t, err, stdout, stderr)
			}

			stdout, stderr, err := e2e.Zarf(t, "dev", "inspect", "definition", packageDir, "-a", arch)
			require.NoError(t, err, stdout, stderr)
			var pkg v1beta1.Package
			require.NoError(t, yaml.Unmarshal([]byte(stdout), &pkg))
			require.Equal(t, v1beta1.ZarfInitConfig, pkg.Kind)
			require.True(t, pkg.Metadata.PreventNamespaceOverride)
			require.Equal(t, arch, pkg.Metadata.Architecture)
			k3s, err := pkg.GetComponent("k3s")
			require.NoError(t, err)
			require.True(t, k3s.Optional)
			require.Equal(t, "linux", k3s.Target.OS)
			require.Contains(t, k3s.Files[3].Source, "k3s-airgap-images-"+arch)
			agent, err := pkg.GetComponent("zarf-agent")
			require.NoError(t, err)
			require.Equal(t, "registry.example:5443/team/agent:custom", agent.Images[0].Name)

			// Publish only to an ephemeral test registry, then discard every repository resource.
			registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
			filenames := map[string]string{
				"k3s":                "k3s/zarf.gen.yaml",
				"zarf-injector":      "injector/zarf.gen.yaml",
				"zarf-seed-registry": "registry/seed-registry.gen.yaml",
				"zarf-registry":      "registry/zarf.gen.yaml",
				"zarf-agent":         "agent/zarf.gen.yaml",
				"git-server":         "git-server/zarf.gen.yaml",
			}
			for i, component := range pkg.Components {
				componentPath := filepath.Join(packageDir, filepath.FromSlash(filenames[component.Name]))
				stdout, stderr, err := e2e.Zarf(t, "component", "publish", componentPath, "oci://"+registryURL, "--plain-http", "-a", arch)
				require.NoError(t, err, stdout, stderr)
				pkg.Components[i] = v1beta1.Component{
					Name: component.Name, Optional: component.Optional,
					ComponentSpec: v1beta1.ComponentSpec{Import: v1beta1.ComponentImport{Remote: []v1beta1.ComponentImportRemote{{
						URL: fmt.Sprintf("oci://%s/%s:%s", registryURL, component.Name, pkg.Metadata.Version),
					}}}},
				}
			}
			pkg.Documentation = nil
			pkg.Values = v1beta1.Values{}
			remoteDefinition, err := yaml.Marshal(pkg)
			require.NoError(t, err)
			remoteDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(remoteDir, "zarf.yaml"), remoteDefinition, 0o600))
			require.NoError(t, os.RemoveAll(filepath.Join(root, "packages")))

			stdout, stderr, err = e2e.Zarf(t, "dev", "inspect", "values-files", remoteDir, "--plain-http", "-a", arch, "--features=values=true",
				"--set-values", "registry.persistence.size=40Gi,gitServer.replicaCount=2",
				"--values", filepath.Join("src", "test", "packages", "16-init-v1beta1", "values.yaml"))
			require.NoError(t, err, stdout, stderr)
			var charts []chartValues
			decoder := yaml.NewDecoder(strings.NewReader(stdout))
			for {
				var values chartValues
				err := decoder.Decode(&values)
				if errors.Is(err, io.EOF) {
					break
				}
				require.NoError(t, err)
				charts = append(charts, values)
			}
			require.Len(t, charts, 4)
			seed, registry, agentValues, git := charts[0], charts[1], charts[2], charts[3]
			require.Equal(t, "127.0.0.1:0/library/registry", seed.Image.Repository)
			require.Equal(t, "127.0.0.1:31999/library/registry", registry.Image.Repository)
			for _, values := range []chartValues{seed, registry} {
				require.Equal(t, "40Gi", values.Persistence.Size)
				htpasswd := values.Secrets.Htpasswd
				require.Contains(t, htpasswd, "\nzarf-pull:")
				require.NotContains(t, htpasswd, `\n`)
			}
			require.Equal(t, "127.0.0.1:31999/team/agent", agentValues.Image.Repository)
			require.Equal(t, "custom", agentValues.Image.Tag)
			require.Equal(t, "infra", agentValues.NodeSelector["role"])
			require.Len(t, agentValues.Tolerations, 1)
			require.Equal(t, 2, git.ReplicaCount)
		})
	}
}

func TestInitV1Beta1GiteaPVC(t *testing.T) {
	// Exercise the actual CLI against an HTTP Kubernetes API; no cluster is required.
	updates := make(chan corev1.PersistentVolumeClaim, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/version":
			if _, err := io.WriteString(w, `{"gitVersion":"v1.34.0"}`); err != nil {
				t.Error(err)
			}
		case "/api/v1/namespaces/zarf/persistentvolumeclaims/custom-gitea":
			if r.Method == http.MethodPut {
				var adoptedPVC corev1.PersistentVolumeClaim
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if _, _, err := scheme.Codecs.UniversalDeserializer().Decode(body, nil, &adoptedPVC); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				updates <- adoptedPVC
			}
			if err := json.NewEncoder(w).Encode(corev1.PersistentVolumeClaim{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"},
				ObjectMeta: metav1.ObjectMeta{Name: "custom-gitea", Namespace: "zarf", Labels: map[string]string{"app": "gitea"}, Annotations: map[string]string{"existing": "true"}},
			}); err != nil {
				t.Error(err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
  - name: test
    cluster:
      server: %s
contexts:
  - name: test
    context:
      cluster: test
      user: test
current-context: test
users:
  - name: test
    user: {}
`, server.URL)
	configPath := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(configPath, []byte(kubeconfig), 0o600))
	t.Setenv("KUBECONFIG", configPath)
	t.Setenv("ZARF_VAR_GIT_SERVER_EXISTING_PVC", "legacy-pvc")
	stdout, stderr, err := e2e.Zarf(t, "internal", "update-gitea-pvc", "--pvc-name", "custom-gitea")
	require.NoError(t, err, stdout, stderr)
	require.Equal(t, "false", stdout)
	select {
	case adoptedPVC := <-updates:
		require.Equal(t, "custom-gitea", adoptedPVC.Name)
		require.Equal(t, "Helm", adoptedPVC.Labels["app.kubernetes.io/managed-by"])
		require.Equal(t, "zarf-gitea", adoptedPVC.Annotations["meta.helm.sh/release-name"])
	default:
		t.Fatal("the requested PVC was not adopted")
	}
}
