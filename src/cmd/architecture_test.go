// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestArchitectureFlagConfigPrecedence(t *testing.T) {
	previous, previousErr := v, vConfigError
	t.Cleanup(func() { v, vConfigError = previous, previousErr })
	for _, tt := range []struct {
		name, config, env, flag, want string
	}{
		{"flag over environment and config", "s390x", "arm64", "amd64", "amd64"},
		{"environment over config", "s390x", "arm64", "", "arm64"},
		{"config", "s390x", "", "", "s390x"},
		{"omitted stays empty", "", "", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "zarf-config.toml")
			require.NoError(t, os.WriteFile(configPath, []byte(fmt.Sprintf("architecture = %q\n", tt.config)), 0o600))
			t.Setenv("ZARF_CONFIG", configPath)
			t.Setenv("ZARF_ARCHITECTURE", tt.env)
			initViper()
			require.NoError(t, vConfigError)
			architecture := &architectureOptions{}
			cmd := &cobra.Command{}
			setupRootFlags(cmd, architecture)
			args := []string{}
			if tt.flag != "" {
				args = append(args, "--architecture", tt.flag)
			}
			require.NoError(t, cmd.ParseFlags(args))
			require.Equal(t, tt.want, architecture.selectedArchitecture(), "parsing must update operation options before execution")
		})
	}
}

func TestArchitectureCommandIsolation(t *testing.T) {
	previous, previousErr := v, vConfigError
	t.Cleanup(func() { v, vConfigError = previous, previousErr })
	configPath := filepath.Join(t.TempDir(), "zarf-config.toml")
	require.NoError(t, os.WriteFile(configPath, nil, 0o600))
	t.Setenv("ZARF_CONFIG", configPath)
	t.Setenv("ZARF_ARCHITECTURE", "")
	initViper()

	target := "arm64"
	if runtime.GOARCH == target {
		target = "amd64"
	}
	source := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, layout.ZarfYAML), []byte(`kind: ZarfPackageConfig
metadata:
  name: isolated
components:
  - name: empty
`), 0o600))
	first := NewZarfCommand()
	require.NoError(t, first.ParseFlags([]string{"--architecture", target}))
	second := NewZarfCommand()

	// Constructing another tree must not reset the first tree's selection.
	for _, tt := range []struct {
		cmd  *cobra.Command
		want string
	}{{first, target}, {second, runtime.GOARCH}} {
		output := t.TempDir()
		create, _, err := tt.cmd.Find([]string{"package", "create"})
		require.NoError(t, err)
		require.NoError(t, create.ParseFlags([]string{"--output", output, "--confirm", "--skip-sbom", "--cache", t.TempDir()}))
		create.SetContext(testutil.TestContext(t))
		require.NoError(t, create.RunE(create, []string{source}))
		require.FileExists(t, filepath.Join(output, "zarf-package-isolated-"+tt.want+".tar.zst"))

		definition, err := load.PackageDefinition(testutil.TestContext(t), source, load.DefinitionOptions{CachePath: t.TempDir()})
		require.NoError(t, err)
		require.Equal(t, runtime.GOARCH, definition.Metadata.Architecture, "library calls must not inherit CLI selection")
	}
}

func TestInitArchitectureDiscovery(t *testing.T) {
	t.Chdir(t.TempDir())
	target := "arm64"
	if runtime.GOARCH == target {
		target = "amd64"
	}
	for _, arch := range []string{target, runtime.GOARCH} {
		name := fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, config.CLIVersion)
		require.NoError(t, os.WriteFile(name, nil, 0o600))
	}
	for _, tt := range []struct{ option, want string }{{target, target}, {"", runtime.GOARCH}} {
		o := initOptions{architectureOptions: &architectureOptions{architecture: tt.option}}
		found, err := o.findInitPackage(testutil.TestContext(t))
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("zarf-init-%s-%s.tar.zst", tt.want, config.CLIVersion), found)
	}
}
