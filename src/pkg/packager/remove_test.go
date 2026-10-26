// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/test/testutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestRemovePersistsChartProgress(t *testing.T) {
	for _, tc := range []struct {
		name      string
		remaining state.InstalledChart
		removed   state.InstalledChart
	}{
		{
			name:      "different releases in one namespace",
			remaining: state.InstalledChart{Namespace: "repro", ChartName: "chart-a"},
			removed:   state.InstalledChart{Namespace: "repro", ChartName: "chart-b"},
		},
		{
			name:      "same release name in different namespaces",
			remaining: state.InstalledChart{Namespace: "repro-a", ChartName: "shared"},
			removed:   state.InstalledChart{Namespace: "repro-b", ChartName: "shared"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.TestContext(t)
			// Helm uses its own Kubernetes client. Simulate a transient API failure for
			// the remaining release and an already-removed release.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/version":
					_, err := w.Write([]byte(`{"gitVersion":"v1.37.0"}`))
					assert.NoError(t, err)
				case "/api/v1/namespaces/" + tc.remaining.Namespace + "/secrets", "/api/v1/namespaces/" + tc.removed.Namespace + "/secrets":
					if r.URL.Path == "/api/v1/namespaces/"+tc.remaining.Namespace+"/secrets" &&
						strings.Contains(r.URL.Query().Get("labelSelector"), "name="+tc.remaining.ChartName) {
						http.Error(w, "release temporarily unavailable", http.StatusForbidden)
						return
					}
					assert.NoError(t, json.NewEncoder(w).Encode(corev1.SecretList{Items: []corev1.Secret{}}))
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(srv.Close)
			kubeconfig := filepath.Join(t.TempDir(), "kubeconfig")
			require.NoError(t, clientcmd.WriteToFile(clientcmdapi.Config{
				Clusters:       map[string]*clientcmdapi.Cluster{"test": {Server: srv.URL}},
				Contexts:       map[string]*clientcmdapi.Context{"test": {Cluster: "test"}},
				CurrentContext: "test",
			}, kubeconfig))
			t.Setenv("KUBECONFIG", kubeconfig)

			c := &cluster.Cluster{Clientset: fake.NewClientset()}
			pkg := api.Package{
				Kind:     api.ZarfPackageConfig,
				Metadata: api.PackageMetadata{Name: "remove-progress"},
				Components: []api.Component{
					{Name: "two-charts", Charts: []api.Chart{
						{Name: "chart-a", Namespace: tc.remaining.Namespace, ReleaseName: tc.remaining.ChartName},
						{Name: "chart-b", Namespace: tc.removed.Namespace, ReleaseName: tc.removed.ChartName},
					}},
				},
			}
			_, err := c.RecordPackageDeployment(ctx, pkg, "", []state.DeployedComponent{
				{Name: "two-charts", InstalledCharts: []state.InstalledChart{
					tc.remaining, tc.removed,
				}},
			}, 1)
			require.NoError(t, err)
			opts := RemoveOptions{Cluster: c}

			// Both the initial failure and a retry must retain only the outstanding chart.
			for range 2 {
				err = Remove(ctx, pkg, opts)
				require.ErrorContains(t, err, "unable to uninstall the helm chart "+tc.remaining.ChartName+" in the namespace "+tc.remaining.Namespace)
				deployed, err := c.GetDeployedPackage(ctx, pkg.Metadata.Name)
				require.NoError(t, err)
				require.Equal(t, []state.DeployedComponent{
					{Name: "two-charts", InstalledCharts: []state.InstalledChart{tc.remaining}},
				}, deployed.DeployedComponents)
			}
		})
	}
}
