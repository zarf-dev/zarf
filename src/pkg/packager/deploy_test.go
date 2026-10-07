// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/template"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
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

func TestProcessComponentFilesCreatesDeclaredSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows symlink creation requires elevated privileges")
	}

	ctx := testutil.TestContext(t)
	packageDir := t.TempDir()
	source := filepath.Join(packageDir, "source.bin")
	content := []byte{0, 0xff}
	require.NoError(t, os.WriteFile(source, content, 0o600))

	destination := filepath.Join(t.TempDir(), "installed.bin")
	symlink := filepath.Join(t.TempDir(), "installed-link")
	definition := fmt.Sprintf(`apiVersion: zarf.dev/v1beta1
kind: ZarfPackageConfig
metadata:
  name: symlink-deploy
components:
  - name: component
    files:
      - source: source.bin
        destination: %q
        symlinks:
          - %q
`, destination, symlink)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, layout.ZarfYAML), []byte(definition), 0o600))

	loaded, err := load.Package(ctx, packageDir, load.PackageOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, loaded.Close())
	})

	pkgLayout, err := assemble.AssemblePackage(ctx, loaded, assemble.AssembleOptions{SkipSBOM: true})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pkgLayout.Cleanup())
	})

	component := pkgLayout.Definition().Components[0]
	err = processComponentFiles(ctx, pkgLayout, component, variables.New("", nil, nil), value.Values{}, template.StateAccess{})
	require.NoError(t, err)

	deployed, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, content, deployed)

	info, err := os.Lstat(symlink)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)
	target, err := os.Readlink(symlink)
	require.NoError(t, err)
	require.Equal(t, destination, target)
}
