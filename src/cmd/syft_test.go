// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestSbomConfigTemporarilyRemovesYQ(t *testing.T) {
	root := &cobra.Command{Use: "zarf"}
	tools := newToolsCommand()
	root.AddCommand(tools)
	root.SetArgs([]string{"tools", "sbom", "config"})
	yqCmd, _, err := tools.Find([]string{"yq"})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		require.NoError(t, root.Execute())
	})

	sbomCmd, _, err := tools.Find([]string{"sbom"})
	require.NoError(t, err)
	require.Same(t, tools, sbomCmd.Parent())
	require.Same(t, tools, yqCmd.Parent())
}
