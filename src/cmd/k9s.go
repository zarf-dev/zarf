// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"flag"

	// Initialize the upstream command accessed through go:linkname below.
	_ "github.com/derailed/k9s/cmd"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"k8s.io/klog/v2"

	// K9s does not export its command constructor. Access its root for independent execution.
	_ "unsafe"
)

//go:linkname k9sRootCmd github.com/derailed/k9s/cmd.rootCmd
var k9sRootCmd *cobra.Command

func newK9sCommand() *cobra.Command {
	cmd := k9sRootCmd
	cmd.Use = "monitor"
	cmd.Aliases = []string{"m", "k9s"}
	cmd.Short = lang.CmdToolsMonitorShort
	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		// Match the logging setup in k9s/main.go without changing the process arguments.
		flags := flag.NewFlagSet("k9s", flag.ContinueOnError)
		klog.InitFlags(flags)
		for _, setting := range [][2]string{
			{"logtostderr", "false"},
			{"alsologtostderr", "false"},
			{"stderrthreshold", "fatal"},
			{"v", "0"},
		} {
			if err := flags.Set(setting[0], setting[1]); err != nil {
				return err
			}
		}
		return nil
	}
	return cmd
}
