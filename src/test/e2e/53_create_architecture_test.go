// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// TestCreateArchitecture is cluster-free and uses only local package resources.
func TestCreateArchitecture(t *testing.T) {
	t.Setenv("ZARF_ARCHITECTURE", "")
	configPath := filepath.Join(t.TempDir(), "zarf-config.toml")
	require.NoError(t, os.WriteFile(configPath, nil, 0o600))
	t.Setenv("ZARF_CONFIG", configPath)
	source := t.TempDir()
	metadataArch := e2e.GetMismatchedArch()
	definition := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: architecture
  architecture: %s
components:
  - name: empty
`, metadataArch)
	require.NoError(t, os.WriteFile(filepath.Join(source, layout.ZarfYAML), []byte(definition), 0o600))
	for _, tt := range []struct {
		name, flag, want string
	}{
		{"metadata fallback", "", metadataArch},
		{"flag override", e2e.Arch, e2e.Arch},
	} {
		t.Run(tt.name, func(t *testing.T) {
			output := t.TempDir()
			args := []string{"package", "create", source, "--output", output, "--confirm", "--skip-sbom", "--cache", t.TempDir()}
			if tt.flag != "" {
				args = append(args, "--architecture", tt.flag)
			}
			_, _, err := e2e.Zarf(t, args...)
			require.NoError(t, err)
			path := filepath.Join(output, "zarf-package-architecture-"+tt.want+".tar.zst")
			pkgLayout, err := layout.LoadFromTar(context.Background(), path, layout.PackageLayoutOptions{})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pkgLayout.Cleanup()) })
			require.Equal(t, tt.want, pkgLayout.Definition().Metadata.Architecture)
			require.Equal(t, tt.want, pkgLayout.Definition().Build.Architecture)
		})
	}
}
