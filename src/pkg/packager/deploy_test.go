// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
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

func TestInternalServicesFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		components []api.Component
		opts       DeployOptions
		expected   state.ServiceSet
	}{
		{
			name:       "no components",
			components: nil,
			expected:   state.NewServiceSet(),
		},
		{
			name: "full init package with no external URLs populates enabled services",
			components: []api.Component{
				{Name: "k3s"},
				{Name: "zarf-injector"},
				{Name: "zarf-seed-registry"},
				{Name: "zarf-registry"},
				{Name: "zarf-agent"},
				{Name: "git-server"},
			},
			expected: state.NewServiceSet(state.RegistryKey, state.AgentKey, state.GitKey),
		},
		{
			name: "external registry URL drops registry key even though registry components are present",
			components: []api.Component{
				{Name: "zarf-injector"},
				{Name: "zarf-seed-registry"},
				{Name: "zarf-registry"},
				{Name: "zarf-agent"},
				{Name: "git-server"},
			},
			opts: DeployOptions{
				RegistryInfo: state.RegistryInfo{Address: "https://registry.example.com"},
			},
			expected: state.NewServiceSet(state.AgentKey, state.GitKey),
		},
		{
			name: "external git and artifact URLs do not change internally deployed services",
			components: []api.Component{
				{Name: "zarf-registry"},
				{Name: "git-server"},
			},
			opts: DeployOptions{
				GitServer:      state.GitServerInfo{Address: "https://git.example.com"},
				ArtifactServer: state.ArtifactServerInfo{Address: "https://artifact.example.com"},
			},
			expected: state.NewServiceSet(state.RegistryKey, state.GitKey),
		},
		{
			name: "registry components dedupe to registry key",
			components: []api.Component{
				{Name: "zarf-injector"},
				{Name: "zarf-seed-registry"},
				{Name: "zarf-registry"},
			},
			expected: state.NewServiceSet(state.RegistryKey),
		},
		{
			name: "unknown components ignored",
			components: []api.Component{
				{Name: "k3s"},
				{Name: "some-custom-component"},
			},
			expected: state.NewServiceSet(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := internalServicesFor(tt.components, tt.opts)
			require.Equal(t, tt.expected, got)
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
	require.ErrorContains(t, err, "--skip-architecture-check is not supported for init packages")
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
