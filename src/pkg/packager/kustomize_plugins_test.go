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
			for _, api := range []string{"inspect", "find-images", "find-definition-images"} {
				t.Run(api, func(t *testing.T) {
					for _, tc := range []struct {
						name           string
						packageEnabled bool
						callerEnabled  bool
					}{
						{name: "disabled by default", packageEnabled: true},
						{name: "explicit opt-in", packageEnabled: true, callerEnabled: true},
						{name: "package setting required", callerEnabled: true},
					} {
						t.Run(tc.name, func(t *testing.T) {
							ctx := testutil.TestContext(t)
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

							var scanErr error
							switch api {
							case "inspect":
								resources, err := InspectDefinitionResources(ctx, path, InspectDefinitionResourcesOptions{EnableKustomizePlugins: tc.callerEnabled})
								scanErr = err
								if err == nil {
									require.Len(t, resources, 1)
									require.Contains(t, resources[0].Content, "name: plugin-pod")
								}
							case "find-images":
								scans, err := FindImages(ctx, path, FindImagesOptions{EnableKustomizePlugins: tc.callerEnabled, SkipCosign: true})
								scanErr = err
								if err == nil {
									require.Len(t, scans, 1)
									require.Len(t, scans[0].Matches, 1)
									require.Equal(t, "docker.io/library/scratch:latest", scans[0].Matches[0].Image.Name)
								}
							case "find-definition-images":
								scans, err := FindDefinitionImages(ctx, path, FindImagesOptions{EnableKustomizePlugins: tc.callerEnabled, SkipCosign: true})
								scanErr = err
								if err == nil {
									require.Len(t, scans, 1)
									require.Len(t, scans[0].Matches, 1)
									require.Equal(t, "docker.io/library/scratch:latest", scans[0].Matches[0].Image.Name)
								}
							}
							if tc.packageEnabled && tc.callerEnabled {
								require.NoError(t, scanErr)
								require.FileExists(t, marker)
							} else {
								require.ErrorContains(t, scanErr, "external plugins disabled")
								if !tc.callerEnabled {
									require.ErrorContains(t, scanErr, "--enable-kustomize-plugins")
								}
								require.NoFileExists(t, marker)
							}
						})
					}
				})
			}
		})
	}
}
