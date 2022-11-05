// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for zarf contains the CLI commands for zarf
package cmd

import (
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/spf13/cobra"
)

var (
	connectResourceName string
	connectNamespace    string
	connectResourceType string
	connectLocalPort    int
	connectRemotePort   int
	cliOnly             bool

	connectCmd = &cobra.Command{
		Use:     lang.CmdConnect,
		Aliases: []string{"c"},
		Short:   lang.CmdConnectShort,
		Long:    lang.CmdConnectLong,
		Run: func(cmd *cobra.Command, args []string) {
			var target string
			if len(args) > 0 {
				target = args[0]
			}

			tunnel := cluster.NewTunnel(connectNamespace, connectResourceType, connectResourceName, connectLocalPort, connectRemotePort)
			// If the cliOnly flag is false (default), enable auto-open
			if !cliOnly {
				tunnel.EnableAutoOpen()
			}
			tunnel.Connect(target, true)
		},
	}

	connectListCmd = &cobra.Command{
		Use:     lang.CmdConnectList,
		Aliases: []string{"l"},
		Short:   lang.CmdConnectListShort,
		Run: func(cmd *cobra.Command, args []string) {
			cluster.NewClusterOrDie().PrintConnectTable()
		},
	}
)

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.AddCommand(connectListCmd)

	connectCmd.Flags().StringVar(&connectResourceName, lang.CmdConnectFlagName, "", lang.CmdConnectFlagNameHelp)
	connectCmd.Flags().StringVar(&connectNamespace, lang.CmdConnectFlagNamespace, cluster.ZarfNamespace, lang.CmdConnectFlagNamespaceHelp)
	connectCmd.Flags().StringVar(&connectResourceType, lang.CmdConnectFlagType, cluster.SvcResource, lang.CmdConnectFlagTypeHelp)
	connectCmd.Flags().IntVar(&connectLocalPort, lang.CmdConnectFlagLocalPort, 0, lang.CmdConnectFlagLocalPortHelp)
	connectCmd.Flags().IntVar(&connectRemotePort, lang.CmdConnectFlagRemotePort, 0, lang.CmdConnectFlagRemotePortHelp)
	connectCmd.Flags().BoolVar(&cliOnly, lang.CmdConnectFlagCliOnly, false, lang.CmdConnectFlagCliOnlyHelp)
}
