// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"

	k9s "github.com/derailed/k9s/cmd"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"

	// This allows for go linkname to be used in this file.  Go linkname is used so that we can pull the CLI flags from k9s and generate proper docs for the vendored tool.
	_ "unsafe"
)

//go:linkname k9sRootCmd github.com/derailed/k9s/cmd.rootCmd
var k9sRootCmd *cobra.Command

func newK9sCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "monitor",
		Aliases: []string{"m", "k9s"},
		Short:   lang.CmdToolsMonitorShort,
		Run: func(_ *cobra.Command, _ []string) {
			// Hack to make k9s think it's all alone
			os.Args = []string{os.Args[0]}
			k9s.Execute()
		},
	}

	cmd.Flags().AddFlagSet(k9sRootCmd.Flags())

	return cmd
}
