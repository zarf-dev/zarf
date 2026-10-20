// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	ptmpl "github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/pki"
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
			components: []api.Component{{Name: "custom-registry", Service: api.Service{Name: api.ServiceRegistry}}},
			registry:   true,
		},
		{
			name: "renamed seed registry",
			components: []api.Component{
				{Name: "probe"},
				{Name: "custom-seed", Service: api.Service{Name: api.ServiceSeedRegistry}},
			},
			registry: true,
		},
		{
			name:       "renamed injector",
			components: []api.Component{{Name: "custom-injector", Service: api.Service{Name: api.ServiceInjector}}},
			registry:   true,
		},
		{
			name:       "renamed agent",
			components: []api.Component{{Name: "custom-agent", Service: api.Service{Name: api.ServiceAgent}}},
			agent:      true,
		},
		{
			name:       "renamed git server",
			components: []api.Component{{Name: "custom-git", Service: api.Service{Name: api.ServiceGitServer}}},
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
				{Name: "custom-registry", Service: api.Service{Name: api.ServiceRegistry}},
				{Name: "custom-seed", Service: api.Service{Name: api.ServiceSeedRegistry}},
				{Name: "custom-injector", Service: api.Service{Name: api.ServiceInjector}},
				{Name: "zarf-registry"},
				{Name: "custom-agent", Service: api.Service{Name: api.ServiceAgent}},
				{Name: "custom-git", Service: api.Service{Name: api.ServiceGitServer}},
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
	err = d.verifyPackageIsDeployable(ctx, &layout.PackageLayout{}, false)
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

func architectureTestLayout(t *testing.T, kind api.PackageKind, withImages bool) *layout.PackageLayout {
	t.Helper()
	component := v1alpha1.ZarfComponent{Name: "application"}
	if withImages {
		component.Images = []string{"example.com/application:latest"}
	}
	pkgLayout := loadTestPackageLayout(t, []v1alpha1.ZarfComponent{component})
	definition := pkgLayout.Definition()
	definition.Kind = kind
	definition.Metadata.Architecture = "amd64"
	require.NoError(t, layout.WritePackageDefinition(filepath.Join(pkgLayout.DirPath(), layout.ZarfYAML), definition))
	pkgLayout, err := layout.LoadFromDir(t.Context(), pkgLayout.DirPath(), layout.PackageLayoutOptions{VerificationStrategy: layout.VerifyNever})
	require.NoError(t, err)
	return pkgLayout
}

func TestVerifyClusterCompatibilityArchitectureOverride(t *testing.T) {
	tests := []struct {
		name       string
		arch       string
		skip       bool
		noImages   bool
		noNodes    bool
		imageIndex string
		err        string
		warn       bool
	}{
		{name: "mismatch rejected", arch: "arm64", err: "this package architecture is amd64"},
		{name: "mismatch allowed", arch: "arm64", skip: true, warn: true},
		{name: "native deployment", arch: "amd64"},
		{name: "native deployment with override", arch: "amd64", skip: true},
		{name: "no images", arch: "arm64", noImages: true},
		{name: "no images with override", arch: "arm64", noImages: true, skip: true},
		{name: "no nodes", noNodes: true, err: lang.ErrUnableToCheckArch.Error()},
		{name: "no nodes with override", noNodes: true, skip: true, err: lang.ErrUnableToCheckArch.Error()},
		{name: "image index", arch: "arm64", imageIndex: `{"manifests":[{"mediaType":"application/vnd.oci.image.index.v1+json"}]}`},
		{name: "image index with override", arch: "arm64", skip: true, imageIndex: `{"manifests":[{"mediaType":"application/vnd.oci.image.index.v1+json"}]}`},
		{name: "malformed image layout", arch: "arm64", skip: true, imageIndex: "invalid", err: "failed to inspect package image layout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgLayout := architectureTestLayout(t, api.ZarfPackageConfig, !tt.noImages)
			if tt.imageIndex != "" {
				require.NoError(t, os.MkdirAll(pkgLayout.GetImageDirPath(), 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(pkgLayout.GetImageDirPath(), layout.IndexJSON), []byte(tt.imageIndex), 0o600))
			}
			cs := fake.NewClientset()
			if !tt.noNodes {
				_, err := cs.CoreV1().Nodes().Create(t.Context(), &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node"},
					Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: tt.arch}},
				}, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			var output bytes.Buffer
			l, err := logger.New(logger.Config{Level: logger.Info, Format: logger.FormatJSON, Destination: logger.Destination(&output)})
			require.NoError(t, err)
			ctx := logger.WithContext(t.Context(), l)
			err = verifyClusterCompatibility(ctx, &cluster.Cluster{Clientset: cs}, pkgLayout, tt.skip)
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
			if tt.noNodes {
				require.ErrorIs(t, err, lang.ErrUnableToCheckArch)
			}
			if tt.warn {
				require.Equal(t, 1, strings.Count(output.String(), "Architecture mismatch allowed."))
				require.Contains(t, output.String(), `"packageArchitecture":"amd64"`)
				require.Contains(t, output.String(), `"nodeArchitectures":["arm64"]`)
				require.Contains(t, output.String(), "not recommended for production")
			} else {
				require.NotContains(t, output.String(), "Architecture mismatch allowed.")
			}
		})
	}
}

func TestDeployRejectsArchitectureOverrideForInit(t *testing.T) {
	pkgLayout := architectureTestLayout(t, api.ZarfInitConfig, true)
	_, err := Deploy(t.Context(), pkgLayout, DeployOptions{SkipArchitectureCheck: true})
	require.ErrorContains(t, err, "skipping architecture checks is not supported for init packages")
}

func TestArchitectureOverridePreservesAgentCertificateCheck(t *testing.T) {
	pkgLayout := architectureTestLayout(t, api.ZarfPackageConfig, true)
	cs := fake.NewClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "arm64-node"},
		Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"}},
	}, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: state.ZarfNamespaceName}})
	c := &cluster.Cluster{Clientset: cs, Watcher: healthchecks.NewImmediateWatcher(status.CurrentStatus)}
	certs, err := pki.GeneratePKIWithOptions("agent", pki.GenerateOptions{Duration: -time.Hour})
	require.NoError(t, err)
	require.NoError(t, c.SaveState(t.Context(), &state.State{AgentInfo: state.AgentInfo{TLS: certs}}))
	d := deployer{c: c}
	err = d.verifyPackageIsDeployable(t.Context(), pkgLayout, true)
	require.ErrorContains(t, err, "the Zarf agent certificate is expired")
}

func TestDeployerActionDefaults(t *testing.T) {
	t.Parallel()

	base := api.ActionDefaults{Env: []string{"FOO=bar"}}

	t.Run("no action env leaves defaults alone", func(t *testing.T) {
		t.Parallel()
		d := deployer{}
		require.Equal(t, []string{"FOO=bar"}, d.actionDefaults(base).Env)
	})

	t.Run("action env is appended last so it wins", func(t *testing.T) {
		t.Parallel()
		d := deployer{actionEnv: []string{"KUBECONFIG=/tmp/kc"}}
		require.Equal(t, []string{"FOO=bar", "KUBECONFIG=/tmp/kc"}, d.actionDefaults(base).Env)
		// The caller's slice must not be aliased, or a second component would accumulate.
		require.Equal(t, []string{"FOO=bar"}, base.Env)
	})
}

// TestDeployActionKubeconfig covers the library path: a caller that selects a cluster through
// DeployOptions gets the same selection in the environment of anything an action shells out to.
// The CLI reaches this through the process environment instead, which is covered in src/cmd.
func TestDeployActionKubeconfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The fixture action reads the variable with POSIX syntax, which PowerShell does not
		// expand. What is under test, the environment reaching the child process, is not
		// OS-specific.
		t.Skip("action fixture uses POSIX shell syntax")
	}

	kubeconfig := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(kubeconfig, []byte("apiVersion: v1\nkind: Config\n"), 0o600))

	seenKubeconfig := func(t *testing.T, opts DeployOptions) string {
		t.Helper()
		ctx := testutil.TestContext(t)
		loaded, err := load.Package(ctx, filepath.Join("testdata", "deploy-action-kubeconfig"), load.PackageOptions{})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, loaded.Close()) })
		pkgLayout, err := assemble.AssemblePackage(ctx, loaded, assemble.AssembleOptions{SkipSBOM: true})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })

		result, err := Deploy(ctx, pkgLayout, opts)
		require.NoError(t, err)
		v, ok := result.VariableConfig.GetSetVariableMap()["SEEN_KUBECONFIG"]
		require.True(t, ok, "the action should have set SEEN_KUBECONFIG")
		return v.Value
	}

	t.Run("no selection leaves the ambient environment alone", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "/ambient/config")
		require.Equal(t, "/ambient/config", seenKubeconfig(t, DeployOptions{}))
	})

	t.Run("a selected kubeconfig reaches the action", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "/ambient/config")
		opts := DeployOptions{ConnectOptions: cluster.ConnectOptions{Kubeconfig: kubeconfig}}
		require.Equal(t, kubeconfig, seenKubeconfig(t, opts))
	})
}
