// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"os"

	"github.com/defenseunicorns/zarf/src/config/lang"
	k9s "github.com/derailed/k9s/cmd"
	"github.com/spf13/cobra"

	// This allows for go linkname to be used in this file.  Go linkname is used so that we can pull the CLI flags from k9s and generate proper docs for the vendored tool.
	_ "unsafe"
)

//go:linkname k9sRootCmd github.com/derailed/k9s/cmd.rootCmd
var k9sRootCmd *cobra.Command

func init() {
	k9sCmd := &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"m", "k9s"},
		Short:   lang.CmdToolsMonitorShort,
		Run: func(_ *cobra.Command, _ []string) {
			// Hack to make k9s think it's all alone
			os.Args = []string{os.Args[0]}
			k9s.Execute()
		},
	}

	k9sCmd.Flags().AddFlagSet(k9sRootCmd.Flags())

	toolsCmd.AddCommand(k9sCmd)
}
