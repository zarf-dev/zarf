// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/cmd/common"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
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
		Use:     "connect { REGISTRY | LOGGING | GIT | connect-name }",
		Aliases: []string{"c"},
		Short:   lang.CmdConnectShort,
		Long:    lang.CmdConnectLong,
		Run: func(cmd *cobra.Command, args []string) {
			var target string
			if len(args) > 0 {
				target = args[0]
			}
			spinner := message.NewProgressSpinner(lang.CmdConnectPreparingTunnel, target)
			c, err := cluster.NewCluster()
			if err != nil {
				spinner.Fatalf(err, lang.CmdConnectErrCluster, err.Error())
			}

			ctx := cmd.Context()

			var tunnel *k8s.Tunnel
			if connectResourceName != "" {
				zt := cluster.NewTunnelInfo(connectNamespace, connectResourceType, connectResourceName, "", connectLocalPort, connectRemotePort)
				tunnel, err = c.ConnectTunnelInfo(ctx, zt)
			} else {
				tunnel, err = c.Connect(ctx, target)
			}
			if err != nil {
				spinner.Fatalf(err, lang.CmdConnectErrService, err.Error())
			}

			defer tunnel.Close()
			url := tunnel.FullURL()

			// Dump the tunnel URL to the console for other tools to use.
			fmt.Print(url)

			if cliOnly {
				spinner.Updatef(lang.CmdConnectEstablishedCLI, url)
			} else {
				spinner.Updatef(lang.CmdConnectEstablishedWeb, url)

				if err := exec.LaunchURL(url); err != nil {
					message.Debug(err)
				}
			}

			// Wait for the interrupt signal or an error.
			select {
			case <-ctx.Done():
				spinner.Successf(lang.CmdConnectTunnelClosed, url)
			case err = <-tunnel.ErrChan():
				spinner.Fatalf(err, lang.CmdConnectErrService, err.Error())
			}
			os.Exit(0)
		},
	}

	connectListCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   lang.CmdConnectListShort,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx := cmd.Context()
			if err := common.NewClusterOrDie(ctx).PrintConnectTable(ctx); err != nil {
				message.Fatal(err, err.Error())
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.AddCommand(connectListCmd)

	connectCmd.Flags().StringVar(&connectResourceName, "name", "", lang.CmdConnectFlagName)
	connectCmd.Flags().StringVar(&connectNamespace, "namespace", cluster.ZarfNamespaceName, lang.CmdConnectFlagNamespace)
	connectCmd.Flags().StringVar(&connectResourceType, "type", k8s.SvcResource, lang.CmdConnectFlagType)
	connectCmd.Flags().IntVar(&connectLocalPort, "local-port", 0, lang.CmdConnectFlagLocalPort)
	connectCmd.Flags().IntVar(&connectRemotePort, "remote-port", 0, lang.CmdConnectFlagRemotePort)
	connectCmd.Flags().BoolVar(&cliOnly, "cli-only", false, lang.CmdConnectFlagCliOnly)
}
