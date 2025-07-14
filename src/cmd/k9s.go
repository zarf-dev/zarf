// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"flag"
	"os"

	k9s "github.com/derailed/k9s/cmd"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"k8s.io/klog/v2"

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
		RunE: func(_ *cobra.Command, _ []string) error {
			// Hack to make k9s think it's all alone
			os.Args = []string{os.Args[0]}

			// Mimic k9s/main.go:init()
			klog.InitFlags(nil)
			if err := flag.Set("logtostderr", "false"); err != nil {
				return err
			}
			if err := flag.Set("alsologtostderr", "false"); err != nil {
				return err
			}
			if err := flag.Set("stderrthreshold", "fatal"); err != nil {
				return err
			}
			if err := flag.Set("v", "0"); err != nil {
				return err
			}

			k9s.Execute()

			return nil
		},
	}

	cmd.Flags().AddFlagSet(k9sRootCmd.Flags())

	return cmd
}
