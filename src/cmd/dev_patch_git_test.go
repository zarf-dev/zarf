// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

func TestDevPatchGitConfirmFlagDefault(t *testing.T) {
	t.Parallel()

	cmd := newDevPatchGitCommand()
	f := cmd.Flags().Lookup("confirm")
	require.NotNil(t, f)
	require.Equal(t, "false", f.DefValue)
}

func TestDevPatchGitWithConfirm(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	original := "repoURL: https://github.com/zarf-dev/zarf.git\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	o := &devPatchGitOptions{
		gitPushUsername: state.ZarfGitPushUser,
		confirm:         true,
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := o.run(cmd, []string{"https://git.example.com", path})
	require.NoError(t, err)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEqual(t, original, string(got))
	require.Contains(t, string(got), "git.example.com")
	require.NotContains(t, string(got), "github.com")
}

func TestDevPatchGitWithConfirmWriteError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	original := "repoURL: https://github.com/zarf-dev/zarf.git\n"
	require.NoError(t, os.WriteFile(path, []byte(original), 0o600))

	// Read-only so the read succeeds but the write back fails.
	require.NoError(t, os.Chmod(path, 0o400))
	t.Cleanup(func() {
		require.NoError(t, os.Chmod(path, 0o600))
	})

	o := &devPatchGitOptions{
		gitPushUsername: state.ZarfGitPushUser,
		confirm:         true,
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := o.run(cmd, []string{"https://git.example.com", path})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unable to write the changes back to the file")
}
