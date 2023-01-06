// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf contains the CLI commands for Zarf.
package cmd

import (
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
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
		Use:     "connect {REGISTRY|LOGGING|GIT|connect-name}",
		Aliases: []string{"c"},
		Short:   lang.CmdConnectShort,
		Long:    lang.CmdConnectLong,
		Run: func(cmd *cobra.Command, args []string) {
			var target string
			if len(args) > 0 {
				target = args[0]
			}

			tunnel, err := cluster.NewTunnel(connectNamespace, connectResourceType, connectResourceName, connectLocalPort, connectRemotePort)
			if err != nil {
				message.Fatal(err, lang.ErrTunnelFailed)
			}
			// If the cliOnly flag is false (default), enable auto-open
			if !cliOnly {
				tunnel.EnableAutoOpen()
			}
			tunnel.Connect(target, true)
		},
	}

	connectListCmd = &cobra.Command{
		Use:     "list",
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

	connectCmd.Flags().StringVar(&connectResourceName, "name", "", lang.CmdConnectFlagName)
	connectCmd.Flags().StringVar(&connectNamespace, "namespace", cluster.ZarfNamespace, lang.CmdConnectFlagNamespace)
	connectCmd.Flags().StringVar(&connectResourceType, "type", cluster.SvcResource, lang.CmdConnectFlagType)
	connectCmd.Flags().IntVar(&connectLocalPort, "local-port", 0, lang.CmdConnectFlagLocalPort)
	connectCmd.Flags().IntVar(&connectRemotePort, "remote-port", 0, lang.CmdConnectFlagRemotePort)
	connectCmd.Flags().BoolVar(&cliOnly, "cli-only", false, lang.CmdConnectFlagCliOnly)
}
