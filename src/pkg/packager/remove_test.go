// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/test/testutil"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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
			deployed, err := c.RecordPackageDeployment(ctx, pkg, "", []state.DeployedComponent{
				{Name: "two-charts", InstalledCharts: []state.InstalledChart{
					tc.remaining, tc.removed,
				}},
			}, 1)
			require.NoError(t, err)
			deployed.SetDeployConfig(DeployConfig{SetVariables: map[string]string{"VARIABLE": "value"}})
			deployed.AppendEvent(state.PackageEvent{Type: state.EventTypeDeploy, Outcome: state.EventOutcomeSucceeded})
			require.NoError(t, c.UpdateDeployedPackage(ctx, *deployed))
			opts := RemoveOptions{Cluster: c}

			// Both the initial failure and a retry must retain only the outstanding chart.
			for range 2 {
				err = Remove(ctx, pkg, opts)
				require.ErrorContains(t, err, "unable to uninstall the helm chart "+tc.remaining.ChartName+" in the namespace "+tc.remaining.Namespace)
				deployed, err := c.GetDeployedPackage(ctx, pkg.Metadata.Name)
				require.NoError(t, err)
				require.Len(t, deployed.DeployedComponents, 1)
				require.Equal(t, []state.InstalledChart{tc.remaining}, deployed.DeployedComponents[0].InstalledCharts)
				require.Equal(t, state.ComponentStatusFailed, deployed.DeployedComponents[0].Status) //nolint:staticcheck // Verify legacy status remains persisted.
				require.Equal(t, state.EventOutcomeFailed, deployed.DeployedComponents[0].LastEvent.Outcome)
				require.Equal(t, state.EventOutcomeInProgress, deployed.Events[len(deployed.Events)-2].Outcome)
				require.Equal(t, state.EventOutcomeFailed, deployed.Events[len(deployed.Events)-1].Outcome)
				require.Equal(t, map[string]string{"VARIABLE": "value"}, deployed.DeployConfig.SetVariables)
			}
		})
	}
}

func TestRemoveSelectsOnlyRequestedComponents(t *testing.T) {
	ctx := testutil.TestContext(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/version":
			_, err := w.Write([]byte(`{"gitVersion":"v1.37.0"}`))
			assert.NoError(t, err)
		default:
			assert.NoError(t, json.NewEncoder(w).Encode(corev1.SecretList{}))
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
		Metadata: api.PackageMetadata{Name: "partial-remove"},
		Components: []api.Component{
			{Name: "one", Charts: []api.Chart{{Name: "one", Namespace: "default", ReleaseName: "one"}}},
			{Name: "two", Charts: []api.Chart{{Name: "two", Namespace: "default", ReleaseName: "two"}}},
		},
	}
	_, err := c.RecordPackageDeployment(ctx, pkg, "sha256:manifest", []state.DeployedComponent{
		{Name: "one", InstalledCharts: []state.InstalledChart{{Namespace: "default", ChartName: "one"}}},
		{Name: "two", InstalledCharts: []state.InstalledChart{{Namespace: "default", ChartName: "two"}}},
	}, 1)
	require.NoError(t, err)

	config := RemoveConfig{OptionalComponents: "one"}
	require.NoError(t, Remove(ctx, pkg, RemoveOptions{Cluster: c, RemoveConfig: config}))

	deployed, err := c.GetDeployedPackage(ctx, pkg.Metadata.Name)
	require.NoError(t, err)
	require.Equal(t, []state.DeployedComponent{{
		Name:            "two",
		InstalledCharts: []state.InstalledChart{{Namespace: "default", ChartName: "two"}},
	}}, deployed.DeployedComponents)
	require.Equal(t, []state.EventOutcome{state.EventOutcomeInProgress, state.EventOutcomeSucceeded}, []state.EventOutcome{
		deployed.Events[0].Outcome,
		deployed.Events[1].Outcome,
	})
	expectedDigest, err := config.Digest()
	require.NoError(t, err)
	require.Equal(t, expectedDigest, deployed.Events[1].ConfigDigest)
}

func TestRemoveLocalPackageDoesNotCreateDeploymentRecord(t *testing.T) {
	t.Parallel()

	for _, withCluster := range []bool{false, true} {
		t.Run(fmt.Sprintf("cluster=%t", withCluster), func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			marker := filepath.Join(t.TempDir(), "removed")
			pkg := api.Package{
				Kind:     api.ZarfPackageConfig,
				Metadata: api.PackageMetadata{Name: "standalone-local"},
				Components: []api.Component{{
					Name: "local",
					Actions: api.ComponentActions{OnRemove: api.ActionSet{
						After: []api.Action{{Cmd: fmt.Sprintf("echo removed > %q", marker)}},
					}},
				}},
			}
			var c *cluster.Cluster
			if withCluster {
				c = &cluster.Cluster{Clientset: fake.NewClientset()}
			}
			require.NoError(t, Remove(ctx, pkg, RemoveOptions{Cluster: c}))
			content, err := os.ReadFile(marker)
			require.NoError(t, err)
			require.Equal(t, "removed", strings.TrimSpace(string(content)))
			if c != nil {
				secrets, err := c.Clientset.CoreV1().Secrets(state.ZarfNamespaceName).List(ctx, metav1.ListOptions{})
				require.NoError(t, err)
				require.Empty(t, secrets.Items, "standalone local removal must not create deployment state")
			}
		})
	}
}

func TestRemoveLocalComponentPreservesRecordedPackage(t *testing.T) {
	t.Parallel()

	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("failure=%t", fail), func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			c := &cluster.Cluster{Clientset: fake.NewClientset()}
			marker := filepath.Join(t.TempDir(), "removed")
			before := []api.Action{{Cmd: fmt.Sprintf("echo removed > %q", marker)}}
			if fail {
				before = append(before, api.Action{Cmd: "exit 1"})
			}
			pkg := api.Package{
				Kind:     api.ZarfPackageConfig,
				Metadata: api.PackageMetadata{Name: "mixed-remove", Version: "1.2.3"},
				Components: []api.Component{
					{Name: "local", Actions: api.ComponentActions{OnRemove: api.ActionSet{Before: before}}},
					{Name: "cluster", Charts: []api.Chart{{Name: "cluster", Namespace: "default", ReleaseName: "cluster"}}},
				},
			}
			remaining := state.DeployedComponent{
				Name:               "cluster",
				ObservedGeneration: 7,
				InstalledCharts:    []state.InstalledChart{{Namespace: "default", ChartName: "cluster"}},
			}
			recorded, err := c.RecordPackageDeployment(ctx, pkg, "sha256:original", []state.DeployedComponent{
				{Name: "local", ObservedGeneration: 7}, remaining,
			}, 7, state.WithPackageNamespaceOverride("tenant"))
			require.NoError(t, err)
			config := state.DeployConfig{SetVariables: map[string]string{"INPUT": "retained"}, NamespaceOverride: "tenant"}
			recorded.SetDeployConfig(config)
			initialEvent := state.PackageEvent{Type: state.EventTypeDeploy, Outcome: state.EventOutcomeSucceeded, Timestamp: time.Unix(1, 0).UTC()}
			recorded.AppendEvent(initialEvent)
			require.NoError(t, c.UpdateDeployedPackage(ctx, *recorded))
			removeConfig := RemoveConfig{OptionalComponents: "local", NamespaceOverride: "tenant"}

			err = Remove(ctx, pkg, RemoveOptions{Cluster: c, RemoveConfig: removeConfig})
			if fail {
				require.ErrorContains(t, err, `command "exit 1" failed`)
			} else {
				require.NoError(t, err)
			}
			content, err := os.ReadFile(marker)
			require.NoError(t, err)
			require.Equal(t, "removed", strings.TrimSpace(string(content)))
			actual, err := c.GetDeployedPackage(ctx, pkg.Metadata.Name, state.WithPackageNamespaceOverride("tenant"))
			require.NoError(t, err)
			require.Equal(t, 7, actual.Generation)
			require.Equal(t, "sha256:original", actual.Digest)
			require.Equal(t, "tenant", actual.NamespaceOverride)
			require.Equal(t, recorded.Data, actual.Data)
			actualConfig, ok := actual.GetDeployConfig()
			require.True(t, ok)
			require.Equal(t, config, actualConfig)
			require.Len(t, actual.Events, 3)
			require.Equal(t, initialEvent, actual.Events[0])
			expectedOutcome := state.EventOutcomeSucceeded
			if fail {
				expectedOutcome = state.EventOutcomeFailed
				require.Len(t, actual.DeployedComponents, 2)
				require.Equal(t, remaining, actual.DeployedComponents[1])
				require.Equal(t, "local", actual.DeployedComponents[0].Name)
				require.Equal(t, state.EventTypeRemove, actual.DeployedComponents[0].LastEvent.Type)
				require.Equal(t, state.EventOutcomeFailed, actual.DeployedComponents[0].LastEvent.Outcome)
				require.Equal(t, 7, actual.DeployedComponents[0].ObservedGeneration)
			} else {
				require.Equal(t, []state.DeployedComponent{remaining}, actual.DeployedComponents)
			}
			require.Equal(t, []state.EventOutcome{state.EventOutcomeSucceeded, state.EventOutcomeInProgress, expectedOutcome},
				[]state.EventOutcome{actual.Events[0].Outcome, actual.Events[1].Outcome, actual.Events[2].Outcome})
			expectedDigest, err := removeConfig.Digest()
			require.NoError(t, err)
			require.Equal(t, expectedDigest, actual.Events[2].ConfigDigest)
			_, err = c.GetDeployedPackage(ctx, pkg.Metadata.Name)
			require.True(t, kerrors.IsNotFound(err), "namespace-scoped removal must not create an unscoped record")
		})
	}
}

func TestRemoveLastLocalComponentDeletesDeploymentRecord(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)
	c := &cluster.Cluster{Clientset: fake.NewClientset()}
	pkg := api.Package{
		Kind:       api.ZarfPackageConfig,
		Metadata:   api.PackageMetadata{Name: "last-local"},
		Components: []api.Component{{Name: "local"}},
	}
	_, err := c.RecordPackageDeployment(ctx, pkg, "sha256:original", []state.DeployedComponent{{Name: "local"}}, 3)
	require.NoError(t, err)
	require.NoError(t, Remove(ctx, pkg, RemoveOptions{Cluster: c}))
	_, err = c.GetDeployedPackage(ctx, pkg.Metadata.Name)
	require.True(t, kerrors.IsNotFound(err), "removing the last recorded local component must delete deployment state")
}

func TestRemoveRejectsUnavailableDeploymentRecord(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name            string
		requiresCluster bool
		lookupError     error
	}{
		{"local forbidden", false, kerrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "zarf-package-lookup-failure", errors.New("denied"))},
		{"local unavailable", false, errors.New("API unavailable")},
		{"cluster not found", true, kerrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, "zarf-package-lookup-failure")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			cs := fake.NewClientset()
			cs.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, k8sruntime.Object, error) {
				return true, nil, tt.lookupError
			})
			marker := filepath.Join(t.TempDir(), "removed")
			component := api.Component{
				Name: "component",
				Actions: api.ComponentActions{OnRemove: api.ActionSet{
					Before: []api.Action{{Cmd: fmt.Sprintf("echo removed > %q", marker)}},
				}},
			}
			if tt.requiresCluster {
				component.Charts = []api.Chart{{Name: "chart", Namespace: "default"}}
			}
			pkg := api.Package{Kind: api.ZarfPackageConfig, Metadata: api.PackageMetadata{Name: "lookup-failure"}, Components: []api.Component{component}}
			err := Remove(ctx, pkg, RemoveOptions{Cluster: &cluster.Cluster{Clientset: cs}})
			require.ErrorIs(t, err, tt.lookupError)
			require.NoFileExists(t, marker, "lookup failures must not run remove actions against synthetic state")
		})
	}
}
