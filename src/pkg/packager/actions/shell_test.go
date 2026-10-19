// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package actions_test

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestRunShellQuote(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"sh", "bash", "pwsh", "powershell"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := exec.LookPath(name); err != nil {
				t.Skipf("%s is not installed", name)
			}
			cmd := "printf '%s' {{ .Shell.Quote .Values.arg }}"
			if name == "pwsh" || name == "powershell" {
				cmd = "Write-Output {{ .Shell.Quote .Values.arg }}"
			}
			for _, arg := range []string{"", "ordinary", "$(touch injected)", "`touch injected`", "a'; touch injected; #", `a"; touch injected; #`} {
				t.Run(arg, func(t *testing.T) {
					t.Parallel()
					dir := t.TempDir()
					vals := value.Values{"arg": arg}
					// An action's shell override must take precedence over its defaults.
					err := actions.Run(testutil.TestContext(t), dir, []api.Action{{
						Cmd:              cmd,
						EnableTemplating: true,
						Shell:            &api.Shell{Linux: name, Darwin: name, Windows: name},
						SetValues:        []api.SetValue{{Key: ".output"}},
					}}, actions.RunOptions{
						DefaultConfig: api.ActionDefaults{Shell: api.Shell{Linux: "unsupported", Darwin: "unsupported", Windows: "unsupported"}},
						Values:        vals,
					})
					require.NoError(t, err)
					require.NoFileExists(t, filepath.Join(dir, "injected"))
					require.Equal(t, arg, vals["output"])
				})
			}
		})
	}
}

func TestRunShellQuoteUnsupportedShell(t *testing.T) {
	t.Parallel()
	err := actions.Run(testutil.TestContext(t), t.TempDir(), []api.Action{{
		Cmd:              "{{ .Shell.Quote .Values.arg }}",
		EnableTemplating: true,
	}}, actions.RunOptions{
		DefaultConfig: api.ActionDefaults{Shell: api.Shell{Linux: "cmd", Darwin: "cmd", Windows: "cmd"}},
		Values:        value.Values{"arg": "$(touch injected)"},
	})
	require.ErrorContains(t, err, `argument quoting is not supported for shell "cmd"`)
}
