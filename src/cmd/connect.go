// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
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

			var tunnel *k8s.Tunnel
			if connectResourceName != "" {
				zt := cluster.NewTunnelInfo(connectNamespace, connectResourceType, connectResourceName, "", connectLocalPort, connectRemotePort)
				tunnel, err = c.ConnectTunnelInfo(zt)
			} else {
				tunnel, err = c.Connect(target)
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

			// Keep this open until an interrupt signal is received.
			interruptChan := make(chan os.Signal, 1)
			signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-interruptChan
				spinner.Successf(lang.CmdConnectTunnelClosed, url)
				os.Exit(0)
			}()

			exec.SuppressGlobalInterrupt = true

			for {
				runtime.Gosched()
			}
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
	connectCmd.Flags().StringVar(&connectNamespace, "namespace", cluster.ZarfNamespaceName, lang.CmdConnectFlagNamespace)
	connectCmd.Flags().StringVar(&connectResourceType, "type", k8s.SvcResource, lang.CmdConnectFlagType)
	connectCmd.Flags().IntVar(&connectLocalPort, "local-port", 0, lang.CmdConnectFlagLocalPort)
	connectCmd.Flags().IntVar(&connectRemotePort, "remote-port", 0, lang.CmdConnectFlagRemotePort)
	connectCmd.Flags().BoolVar(&cliOnly, "cli-only", false, lang.CmdConnectFlagCliOnly)
}
