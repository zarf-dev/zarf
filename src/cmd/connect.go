// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf contains the CLI commands for Zarf.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

var (
	connectResourceName string
	connectNamespace    string
	connectResourceType string
	connectLocalPort    int
	connectRemotePort   int
	cliOnly             bool
)

var connectCmd = &cobra.Command{
	Use:     "connect { REGISTRY | GIT | connect-name }",
	Aliases: []string{"c"},
	Short:   lang.CmdConnectShort,
	Long:    lang.CmdConnectLong,
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if len(args) > 0 {
			target = args[0]
		}

		spinner := message.NewProgressSpinner(lang.CmdConnectPreparingTunnel, target)
		defer spinner.Stop()

		c, err := cluster.NewCluster()
		if err != nil {
			return err
		}

		ctx := cmd.Context()

		var tunnel *cluster.Tunnel
		if connectResourceName == "" {
			tunnel, err = c.Connect(ctx, target)
		} else {
			zt := cluster.NewTunnelInfo(connectNamespace, connectResourceType, connectResourceName, "", connectLocalPort, connectRemotePort)
			tunnel, err = c.ConnectTunnelInfo(ctx, zt)
		}
		if err != nil {
			return fmt.Errorf("unable to connect to the service: %w", err)
		}
		defer tunnel.Close()

		// Dump the tunnel URL to the console for other tools to use.
		fmt.Print(tunnel.FullURL())

		if cliOnly {
			spinner.Updatef(lang.CmdConnectEstablishedCLI, tunnel.FullURL())
		} else {
			spinner.Updatef(lang.CmdConnectEstablishedWeb, tunnel.FullURL())
			if err := exec.LaunchURL(tunnel.FullURL()); err != nil {
				message.Debug(err)
			}
		}

		select {
		case <-ctx.Done():
			spinner.Successf(lang.CmdConnectTunnelClosed, tunnel.FullURL())
			return nil
		case err = <-tunnel.ErrChan():
			return fmt.Errorf("lost connection to the service: %w", err)
		}
	},
}

var connectListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l"},
	Short:   lang.CmdConnectListShort,
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := cluster.NewCluster()
		if err != nil {
			return err
		}
		connections, err := c.ListConnections(cmd.Context())
		if err != nil {
			return err
		}
		message.PrintConnectStringTable(connections)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
	connectCmd.AddCommand(connectListCmd)

	connectCmd.Flags().StringVar(&connectResourceName, "name", "", lang.CmdConnectFlagName)
	connectCmd.Flags().StringVar(&connectNamespace, "namespace", cluster.ZarfNamespaceName, lang.CmdConnectFlagNamespace)
	connectCmd.Flags().StringVar(&connectResourceType, "type", cluster.SvcResource, lang.CmdConnectFlagType)
	connectCmd.Flags().IntVar(&connectLocalPort, "local-port", 0, lang.CmdConnectFlagLocalPort)
	connectCmd.Flags().IntVar(&connectRemotePort, "remote-port", 0, lang.CmdConnectFlagRemotePort)
	connectCmd.Flags().BoolVar(&cliOnly, "cli-only", false, lang.CmdConnectFlagCliOnly)
}
