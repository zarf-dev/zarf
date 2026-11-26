// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestSbomConfig(t *testing.T) {
	root := &cobra.Command{Use: "zarf"}
	tools := newToolsCommand(&architectureOptions{})
	root.AddCommand(tools)
	root.SetArgs([]string{"tools", "sbom", "config"})
	root.SetOut(io.Discard)

	// The Syft config summary walks the flags of every command in the tree, including yq's.
	_, _, err := tools.Find([]string{"yq"})
	require.NoError(t, err)

	require.NotPanics(t, func() {
		require.NoError(t, root.Execute())
	})
}
