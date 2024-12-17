// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf contains the CLI commands for Zarf.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

// ConnectOptions holds the command-line options for 'connect' sub-command.
type ConnectOptions struct {
	cliOnly bool
	zt      cluster.TunnelInfo
}

// NewConnectCommand creates the `connect` sub-command and its nested children.
func NewConnectCommand() *cobra.Command {
	o := &ConnectOptions{}

	cmd := &cobra.Command{
		Use:     "connect { REGISTRY | GIT | connect-name }",
		Aliases: []string{"c"},
		Short:   lang.CmdConnectShort,
		Long:    lang.CmdConnectLong,
		RunE:    o.Run,
	}

	cmd.Flags().StringVar(&o.zt.ResourceName, "name", "", lang.CmdConnectFlagName)
	cmd.Flags().StringVar(&o.zt.Namespace, "namespace", cluster.ZarfNamespaceName, lang.CmdConnectFlagNamespace)
	cmd.Flags().StringVar(&o.zt.ResourceType, "type", cluster.SvcResource, lang.CmdConnectFlagType)
	cmd.Flags().IntVar(&o.zt.LocalPort, "local-port", 0, lang.CmdConnectFlagLocalPort)
	cmd.Flags().IntVar(&o.zt.RemotePort, "remote-port", 0, lang.CmdConnectFlagRemotePort)
	cmd.Flags().BoolVar(&o.cliOnly, "cli-only", false, lang.CmdConnectFlagCliOnly)

	// TODO(soltysh): consider splitting sub-commands into separate files
	cmd.AddCommand(NewConnectListCommand())

	return cmd
}

// Run performs the execution of 'connect' sub command.
func (o *ConnectOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	l := logger.From(ctx)
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

	var tunnel *cluster.Tunnel
	if target == "" {
		tunnel, err = c.ConnectTunnelInfo(ctx, o.zt)
	} else {
		var ti cluster.TunnelInfo
		ti, err = c.NewTargetTunnelInfo(ctx, target)
		if err != nil {
			return fmt.Errorf("unable to create tunnel: %w", err)
		}
		if o.zt.LocalPort != 0 {
			ti.LocalPort = o.zt.LocalPort
		}
		tunnel, err = c.ConnectTunnelInfo(ctx, ti)
	}

	if err != nil {
		return fmt.Errorf("unable to connect to the service: %w", err)
	}

	defer tunnel.Close()

	if o.cliOnly {
		spinner.Updatef(lang.CmdConnectEstablishedCLI, tunnel.FullURL())
		l.Info("Tunnel established, waiting for user to interrupt (ctrl-c to end)", "url", tunnel.FullURL())
	} else {
		spinner.Updatef(lang.CmdConnectEstablishedWeb, tunnel.FullURL())
		l.Info("Tunnel established, opening your default web browser (ctrl-c to end)", "url", tunnel.FullURL())
		if err := exec.LaunchURL(tunnel.FullURL()); err != nil {
			return err
		}
	}

	// Wait for the interrupt signal or an error.
	select {
	case <-ctx.Done():
		spinner.Successf(lang.CmdConnectTunnelClosed, tunnel.FullURL())
		return nil
	case err = <-tunnel.ErrChan():
		return fmt.Errorf("lost connection to the service: %w", err)
	}
}

// ConnectListOptions holds the command-line options for 'connect list' sub-command.
type ConnectListOptions struct{}

// NewConnectListCommand creates the `connect list` sub-command.
func NewConnectListCommand() *cobra.Command {
	o := &ConnectListOptions{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   lang.CmdConnectListShort,
		RunE:    o.Run,
	}
	return cmd
}

// Run performs the execution of 'connect list' sub-command.
func (o *ConnectListOptions) Run(cmd *cobra.Command, _ []string) error {
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
}
