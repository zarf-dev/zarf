// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestKustomizePluginsRequireOptIn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test plugin requires a POSIX shell")
	}

	fixture, err := filepath.Abs(filepath.Join("testdata", "kustomize-plugins"))
	require.NoError(t, err)
	t.Setenv("KUSTOMIZE_PLUGIN_HOME", filepath.Join(fixture, "plugins"))

	for _, definition := range []string{"zarf.yaml", "zarf-v1beta1.yaml", "component.yaml"} {
		t.Run(definition, func(t *testing.T) {
			for _, tc := range []struct {
				name           string
				packageEnabled bool
				callerEnabled  bool
				wantError      string
			}{
				{name: "caller opt-in required", packageEnabled: true, wantError: "requires explicit opt-in"},
				{name: "explicit opt-in", packageEnabled: true, callerEnabled: true},
				{name: "package setting required", callerEnabled: true, wantError: "external plugins disabled"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					dir := t.TempDir()
					require.NoError(t, os.CopyFS(dir, os.DirFS(fixture)))
					path := filepath.Join(dir, definition)
					if !tc.packageEnabled {
						b, err := os.ReadFile(path)
						require.NoError(t, err)
						require.NoError(t, os.WriteFile(path, []byte(strings.ReplaceAll(string(b), ": true", ": false")), 0o600))
					}
					marker := filepath.Join(t.TempDir(), "executed")
					t.Setenv("ZARF_TEST_KUSTOMIZE_PLUGIN_MARKER", marker)

					resources, err := InspectDefinitionResources(testutil.TestContext(t), path, InspectDefinitionResourcesOptions{EnableKustomizePlugins: tc.callerEnabled})
					if tc.wantError != "" {
						require.ErrorContains(t, err, tc.wantError)
						require.NoFileExists(t, marker)
						return
					}
					require.NoError(t, err)
					require.Len(t, resources, 1)
					require.Contains(t, resources[0].Content, "name: plugin-pod")
					require.FileExists(t, marker)
				})
			}
		})
	}
}
