// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	ptmpl "github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/test/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

func TestDeploySuccessHooksPersistStatusByAPIVersion(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		apiVersion        string
		duringHook, final state.ComponentStatus
	}{
		{v1alpha1.APIVersion, state.ComponentStatusSucceeded, state.ComponentStatusSucceeded},
		{v1beta1.APIVersion, state.ComponentStatusDeploying, state.ComponentStatusFailed},
	} {
		t.Run(tt.apiVersion, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			c := &cluster.Cluster{Clientset: fake.NewClientset()}
			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				deployed, err := c.GetDeployedPackage(ctx, "success-hooks")
				if assert.NoError(t, err) {
					assert.Equal(t, tt.duringHook, deployed.DeployedComponents[0].Status)
				}
			}))
			t.Cleanup(srv.Close)
			timeout := 5
			pkg := api.Package{
				APIVersion: tt.apiVersion,
				Kind:       api.ZarfPackageConfig,
				Metadata:   api.PackageMetadata{Name: "success-hooks"},
				Components: []api.Component{{Name: "component", Actions: api.ComponentActions{OnDeploy: api.ActionSet{
					OnSuccess: []api.Action{
						{MaxTotalSeconds: &timeout, Wait: &api.ActionWait{Network: &api.ActionWaitNetwork{Protocol: "http", Address: srv.Listener.Addr().String()}}},
						{Cmd: "exit 1"},
					},
				}}}},
			}
			pkgLayout, err := assemble.AssemblePackage(ctx, &load.ResolvedPackage{Definition: pkg, Resources: load.NewResourceSet(t.TempDir())}, assemble.AssembleOptions{SkipSBOM: true})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
			d := deployer{c: c, vc: ptmpl.GetZarfVariableConfig(ctx, false)}
			_, err = d.deployComponents(ctx, pkgLayout, DeployOptions{})
			require.ErrorContains(t, err, `command "exit 1" failed`)
			deployed, err := c.GetDeployedPackage(ctx, pkg.Metadata.Name)
			require.NoError(t, err)
			require.Equal(t, tt.final, deployed.DeployedComponents[0].Status)
		})
	}
}

func TestDeployInitComponentServices(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name            string
		components      []api.Component
		opts            DeployOptions
		registry        bool
		agent           bool
		git             bool
		failedComponent int
	}{
		{
			name:       "renamed registry",
			components: []api.Component{{Name: "custom-registry", Service: api.ServiceRegistry}},
			registry:   true,
		},
		{
			name: "renamed seed registry",
			components: []api.Component{
				{Name: "probe"},
				{Name: "custom-seed", Service: api.ServiceSeedRegistry},
			},
			registry: true,
		},
		{
			name:       "renamed injector",
			components: []api.Component{{Name: "custom-injector", Service: api.ServiceInjector}},
			registry:   true,
		},
		{
			name:       "renamed agent",
			components: []api.Component{{Name: "custom-agent", Service: api.ServiceAgent}},
			agent:      true,
		},
		{
			name:       "renamed git server",
			components: []api.Component{{Name: "custom-git", Service: api.ServiceGitServer}},
			git:        true,
		},
		{
			name: "legacy names without services",
			components: []api.Component{
				{Name: "zarf-registry"},
				{Name: "zarf-seed-registry"},
				{Name: "zarf-injector"},
				{Name: "zarf-agent"},
				{Name: "git-server"},
			},
		},
		{
			name: "external registry skips renamed registry services",
			components: []api.Component{
				{Name: "custom-registry", Service: api.ServiceRegistry},
				{Name: "custom-seed", Service: api.ServiceSeedRegistry},
				{Name: "custom-injector", Service: api.ServiceInjector},
				{Name: "zarf-registry"},
				{Name: "custom-agent", Service: api.ServiceAgent},
				{Name: "custom-git", Service: api.ServiceGitServer},
			},
			opts:            DeployOptions{RegistryInfo: state.RegistryInfo{Address: "registry.example.com"}},
			registry:        true,
			agent:           true,
			git:             true,
			failedComponent: 3,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			cs := fake.NewClientset(
				&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node"}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName}},
				&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: state.ZarfNamespaceName}},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: "zarf-ip-family-test", Namespace: state.ZarfNamespaceName},
					Spec:       corev1.ServiceSpec{IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol}},
				},
			)
			c := &cluster.Cluster{Clientset: cs, Watcher: healthchecks.NewImmediateWatcher(status.CurrentStatus)}
			for i := range tt.components {
				tt.components[i].Manifests = []api.Manifest{{Name: "probe", Files: []string{"manifest.yaml"}}}
				tt.components[i].Actions.OnDeploy.Before = []api.Action{{Cmd: "exit 1"}}
			}
			pkg := api.Package{
				APIVersion: v1beta1.APIVersion,
				Kind:       api.ZarfInitConfig,
				Metadata:   api.PackageMetadata{Name: "service-init"},
				Components: tt.components,
			}
			dir := t.TempDir()
			require.NoError(t, layout.WritePackageDefinition(filepath.Join(dir, "zarf.yaml"), pkg))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: probe\n"), 0o600))
			loaded, err := load.Package(ctx, dir, load.PackageOptions{})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, loaded.Close()) })
			pkgLayout, err := assemble.AssemblePackage(ctx, loaded, assemble.AssembleOptions{SkipSBOM: true})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
			d := deployer{c: c, vc: ptmpl.GetZarfVariableConfig(ctx, false)}
			// Stop before installing resources so the persisted init state and skipped services
			// can be checked without a live Helm connection.
			_, err = d.deployComponents(ctx, pkgLayout, tt.opts)
			require.ErrorContains(t, err, `unable to deploy component "`+tt.components[tt.failedComponent].Name+`"`)
			require.ErrorContains(t, err, `command "exit 1" failed`)

			s, err := c.LoadState(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.registry, s.RegistryInfo.IsConfigured())
			require.Equal(t, tt.agent, s.AgentInfo.IsConfigured())
			require.Equal(t, tt.git, s.GitServer.IsConfigured())
			if tt.opts.RegistryInfo.Address != "" {
				require.Equal(t, tt.opts.RegistryInfo.Address, s.RegistryInfo.Address)
			}
		})
	}
}

func TestVerifyPackageIsDeployableSkipsAgentCertCheckWhenAgentIsNotConfigured(t *testing.T) {
	ctx := context.Background()
	cs := fake.NewClientset()
	c := &cluster.Cluster{
		Clientset: cs,
		Watcher:   healthchecks.NewImmediateWatcher(status.CurrentStatus),
	}
	_, err := cs.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, c.SaveState(ctx, &state.State{}))

	d := deployer{c: c}
	err = d.verifyPackageIsDeployable(ctx, &layout.PackageLayout{})
	require.NoError(t, err)
}

func TestDeploySkipsValuesSchemaValidationWhenConfigured(t *testing.T) {
	ctx := testutil.TestContext(t)
	srcDir := filepath.Join("load", "testdata", "package-with-invalid-values")
	loaded, err := load.Package(ctx, srcDir, load.PackageOptions{SkipValuesSchemaValidation: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, loaded.Close()) })
	pkgLayout, err := assemble.AssemblePackage(ctx, loaded, assemble.AssembleOptions{SkipSBOM: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

	_, err = Deploy(ctx, pkgLayout, DeployOptions{})
	require.ErrorContains(t, err, "values validation failed")

	_, err = Deploy(ctx, pkgLayout, DeployOptions{SkipValuesSchemaValidation: true})
	require.NoError(t, err)
}
