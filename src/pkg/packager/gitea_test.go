// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/convert"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	itpl "github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/template"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/test/testutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestGiteaTemplates(t *testing.T) {
	t.Parallel()
	packageDir := filepath.Join("..", "..", "..", "packages", "gitea")
	definition, err := os.ReadFile(filepath.Join(packageDir, "zarf.yaml"))
	require.NoError(t, err)
	var authored v1alpha1.ZarfPackage
	require.NoError(t, yaml.Unmarshal(definition, &authored))
	pkg := convert.PackageFromV1alpha1(authored)
	require.Len(t, pkg.Components, 1)
	component := pkg.Components[0]
	require.Len(t, component.Charts, 1)
	require.Len(t, component.Charts[0].ValuesFiles, 1)
	require.Len(t, component.Manifests, 1)
	require.Len(t, component.Manifests[0].Files, 1)

	for _, tc := range []struct {
		name     string
		mode     state.GitTLSMode
		address  string
		legacy   bool
		protocol string
		optional bool
	}{
		{name: "older CLI without TLS field", legacy: true, protocol: "http", optional: true},
		{name: "legacy state", protocol: "http", optional: true},
		{name: "TLS disabled", mode: state.GitTLSDisabled, protocol: "http", optional: true},
		{name: "TLS enabled", mode: state.GitTLSEnabled, protocol: "https"},
		{name: "external HTTPS Git", address: "https://git.example.com", protocol: "http", optional: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := testutil.TestContext(t)
			s := &state.State{GitServer: state.GitServerInfo{TLSMode: tc.mode, Address: tc.address}}
			require.NoError(t, s.GitServer.FillInEmptyValues())
			objs, err := template.NewObjects(value.Values{}).WithState(template.StateAccess{State: s})
			require.NoError(t, err)
			if tc.legacy {
				// Model the public template fields exposed by CLIs before Git TLS.
				objs = template.Objects{"State": map[string]any{"Git": map[string]any{
					"Address": s.GitServer.Address, "IsInternal": true,
				}}}
			} else {
				out, err := template.Apply(ctx, `{{ .State.Git.TLSEnabled }}`, objs)
				require.NoError(t, err)
				require.Equal(t, strconv.FormatBool(tc.protocol == "https"), out)
			}
			vc := variables.New("ZARF", nil, nil)
			require.NoError(t, vc.PopulateVariables(pkg.Variables, nil))
			vc.SetVariable("GIT_SERVER_CREATE_PVC", "false", false, false, "")
			vc.SetConstants(pkg.Constants)
			builtins, err := itpl.GetZarfTemplates(ctx, component.Name, s)
			require.NoError(t, err)
			if tc.legacy {
				builtins = map[string]*variables.TextTemplate{
					"###ZARF_STORAGE_CLASS###": {Value: s.StorageClass},
					"###ZARF_GIT_PUSH###":      {Value: s.GitServer.PushUsername},
					"###ZARF_GIT_AUTH_PUSH###": {Value: s.GitServer.PushPassword},
					"###ZARF_GIT_PULL###":      {Value: s.GitServer.PullUsername},
					"###ZARF_GIT_AUTH_PULL###": {Value: s.GitServer.PullPassword},
				}
			}
			vc.SetApplicationTemplates(builtins)

			valuesFile := component.Charts[0].ValuesFiles[0]
			valuesPath := filepath.Join(t.TempDir(), "values.yaml")
			content, err := os.ReadFile(filepath.Join(packageDir, valuesFile.Path))
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(valuesPath, content, 0o600))
			require.NoError(t, vc.ReplaceTextTemplate(valuesPath))
			if valuesFile.EnableTemplating {
				require.NoError(t, template.ApplyToFile(ctx, valuesPath, valuesPath, objs))
			}
			content, err = os.ReadFile(valuesPath)
			require.NoError(t, err)
			var values struct {
				Gitea struct {
					Config struct {
						Server struct {
							Protocol string `json:"PROTOCOL"`
							RootURL  string `json:"ROOT_URL"`
						} `json:"server"`
					} `json:"config"`
				} `json:"gitea"`
				ExtraVolumes []corev1.Volume `json:"extraVolumes"`
			}
			require.NoError(t, yaml.Unmarshal(content, &values))
			require.Equal(t, tc.protocol, values.Gitea.Config.Server.Protocol)
			require.Equal(t, tc.protocol+"://"+state.ZarfInClusterGitServiceHost+":3000", values.Gitea.Config.Server.RootURL)
			require.Len(t, values.ExtraVolumes, 1)
			require.NotNil(t, values.ExtraVolumes[0].Secret)
			require.Equal(t, state.GitServerTLSSecret, values.ExtraVolumes[0].Secret.SecretName)
			require.NotNil(t, values.ExtraVolumes[0].Secret.Optional)
			require.Equal(t, tc.optional, *values.ExtraVolumes[0].Secret.Optional)

			manifest := component.Manifests[0]
			content, err = os.ReadFile(filepath.Join(packageDir, manifest.Files[0]))
			require.NoError(t, err)
			rendered := string(content)
			if manifest.EnableTemplating {
				rendered, err = template.Apply(ctx, rendered, objs)
				require.NoError(t, err)
			}
			var service corev1.Service
			require.NoError(t, yaml.Unmarshal([]byte(rendered), &service))
			require.Equal(t, tc.protocol, service.Annotations["zarf.dev/connect-scheme"])
		})
	}
}
