// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/spf13/cobra"

	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
)

type connectOptions struct {
	open bool
	zt   cluster.TunnelInfo
}

func newConnectCommand() *cobra.Command {
	o := &connectOptions{}

	cmd := &cobra.Command{
		Use:     "connect { REGISTRY | GIT | connect-name }",
		Aliases: []string{"c"},
		Short:   lang.CmdConnectShort,
		Long:    lang.CmdConnectLong,
		RunE:    o.run,
	}

	cmd.Flags().StringSliceVar(&o.zt.ListenAddresses, "address", []string{helpers.IPV4Localhost}, lang.CmdConnectFlagAddress)
	cmd.Flags().StringVar(&o.zt.ResourceName, "name", "", lang.CmdConnectFlagName)
	cmd.Flags().StringVar(&o.zt.Namespace, "namespace", state.ZarfNamespaceName, lang.CmdConnectFlagNamespace)
	cmd.Flags().StringVar(&o.zt.ResourceType, "type", cluster.SvcResource, lang.CmdConnectFlagType)
	cmd.Flags().IntVar(&o.zt.LocalPort, "local-port", 0, lang.CmdConnectFlagLocalPort)
	cmd.Flags().IntVar(&o.zt.RemotePort, "remote-port", 0, lang.CmdConnectFlagRemotePort)
	cmd.Flags().BoolVar(&o.open, "open", false, lang.CmdConnectFlagOpen)

	// TODO(soltysh): consider splitting sub-commands into separate files
	cmd.AddCommand(newConnectListCommand())

	return cmd
}

func (o *connectOptions) run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	l := logger.From(ctx)
	target := ""
	// TODO: this leaves room for ignoring potential misuse
	if len(args) > 0 {
		target = args[0]
	}

	c, err := cluster.New(ctx)
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
		ti.ListenAddresses = o.zt.ListenAddresses

		tunnel, err = c.ConnectTunnelInfo(ctx, ti)
	}

	if err != nil {
		return fmt.Errorf("unable to connect to the service: %w", err)
	}

	defer tunnel.Close()

	if o.open {
		urls := tunnel.FullURLs()
		if len(urls) == 0 {
			return fmt.Errorf("no tunnel URLs found")
		}
		// Open the first URL (arbitrary)
		l.Info("Tunnel established, opening your default web browser (ctrl-c to end)", "urls", strings.Join(urls, ", "))
		if err := exec.LaunchURL(urls[0]); err != nil {
			return err
		}
	} else {
		urls := tunnel.FullURLs()
		if len(urls) == 0 {
			return fmt.Errorf("no tunnel URLs found")
		}
		l.Info("Tunnel established, waiting for user to interrupt (ctrl-c to end)", "urls", strings.Join(urls, ", "))
	}

	// Wait for the interrupt signal or an error.
	select {
	case <-ctx.Done():
		return nil
	case err = <-tunnel.ErrChan():
		return fmt.Errorf("lost connection to the service: %w", err)
	}
}

// connectListOptions holds the command-line options for 'connect list' sub-command.
type connectListOptions struct{}

// newConnectListCommand creates the `connect list` sub-command.
func newConnectListCommand() *cobra.Command {
	o := &connectListOptions{}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"l", "ls"},
		Short:   lang.CmdConnectListShort,
		RunE:    o.run,
	}
	return cmd
}

func (o *connectListOptions) run(cmd *cobra.Command, _ []string) error {
	c, err := cluster.New(cmd.Context())
	if err != nil {
		return err
	}
	connections, err := c.ListConnections(cmd.Context())
	if err != nil {
		return err
	}
	printConnectStringTable(connections)
	return nil
}

func printConnectStringTable(connectStrings state.ConnectStrings) {
	if len(connectStrings) > 0 {
		connectData := [][]string{}
		// Loop over each connectStrings and convert to a string matrix
		for name, connect := range connectStrings {
			name = fmt.Sprintf("zarf connect %s", name)
			connectData = append(connectData, []string{name, connect.Description})
		}

		// Create the table output with the data
		header := []string{"Connect Command", "Description"}
		message.TableWithWriter(OutputWriter, header, connectData)
	}
}
