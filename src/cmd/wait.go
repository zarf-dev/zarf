// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/wait"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func newWaitForCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "wait-for COMMAND",
		Aliases: []string{"w", "wait"},
		Short:   lang.CmdToolsWaitForShort,
		Long:    lang.CmdToolsWaitForLong,
		Example: lang.CmdToolsWaitForExample,
		Args:    cobra.NoArgs,
		RunE:    func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newWaitForResourceCommand())
	cmd.AddCommand(newWaitForNetworkCommand())
	return cmd
}

type waitForResourceOptions struct {
	timeout   string
	namespace string
}

func newWaitForResourceCommand() *cobra.Command {
	o := waitForResourceOptions{}
	cmd := &cobra.Command{
		Use:     "resource KIND [NAME | SELECTOR] [CONDITION]",
		Short:   lang.CmdToolsWaitForResourceShort,
		Long:    lang.CmdToolsWaitForResourceLong,
		Example: lang.CmdToolsWaitForResourceExample,
		Args:    cobra.RangeArgs(1, 3),
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&o.timeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
	cmd.Flags().StringVarP(&o.namespace, "namespace", "n", "", lang.CmdToolsWaitForFlagNamespace)

	return cmd
}

func (o *waitForResourceOptions) run(cmd *cobra.Command, args []string) error {
	timeout, err := time.ParseDuration(o.timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout duration %s, use a valid duration string e.g. 1s, 2m, 3h: %w", o.timeout, err)
	}

	kind := args[0]
	// identifier is optional to allow for waiting for any resource of a kind to exist in cluster. For instance, `zarf tools wait-for resource storageclass`
	identifier := ""
	if len(args) > 1 {
		identifier = args[1]
	}

	condition := ""
	if len(args) > 2 {
		condition = args[2]
	}

	return wait.ForResourceDefaultReady(cmd.Context(), kind, identifier, condition, o.namespace, timeout)
}

type waitForNetworkOptions struct {
	timeout string
}

func newWaitForNetworkCommand() *cobra.Command {
	o := waitForNetworkOptions{}
	cmd := &cobra.Command{
		Use:     "network PROTOCOL ADDRESS [CODE]",
		Short:   lang.CmdToolsWaitForNetworkShort,
		Long:    lang.CmdToolsWaitForNetworkLong,
		Example: lang.CmdToolsWaitForNetworkExample,
		Args:    cobra.RangeArgs(2, 3),
		RunE:    o.run,
	}

	cmd.Flags().StringVar(&o.timeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)

	return cmd
}

func (o *waitForNetworkOptions) run(cmd *cobra.Command, args []string) error {
	timeout, err := time.ParseDuration(o.timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout duration %s, use a valid duration string e.g. 1s, 2m, 3h: %w", o.timeout, err)
	}

	protocol := args[0]
	address := args[1]

	condition := ""
	if len(args) > 2 {
		condition = args[2]
	}

	return wait.ForNetwork(cmd.Context(), protocol, address, condition, timeout)
}
