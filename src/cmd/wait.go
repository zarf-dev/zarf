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

type waitForOptions struct {
	waitTimeout   string
	waitNamespace string
}

func newWaitForCommand() *cobra.Command {
	o := waitForOptions{}
	cmd := &cobra.Command{
		Use:     "wait-for { KIND | PROTOCOL } { NAME | SELECTOR | URI } { CONDITION | HTTP_CODE }",
		Aliases: []string{"w", "wait"},
		Short:   lang.CmdToolsWaitForShort,
		Long:    lang.CmdToolsWaitForLong,
		Example: lang.CmdToolsWaitForExample,
		Args:    cobra.MinimumNArgs(1),
		RunE:    o.run,
	}
	cmd.AddCommand(newWaitForResourceCommand())
	cmd.AddCommand(newWaitForNetworkCommand())
	cmd.Deprecated = "use `zarf tools wait-for-resource` or `zarf tools wait-for network` instead."

	cmd.Flags().StringVar(&o.waitTimeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
	cmd.Flags().StringVarP(&o.waitNamespace, "namespace", "n", "", lang.CmdToolsWaitForFlagNamespace)

	return cmd
}

func (o *waitForOptions) run(cmd *cobra.Command, args []string) error {
	// Parse the timeout string
	timeout, err := time.ParseDuration(o.waitTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout duration %s, use a valid duration string e.g. 1s, 2m, 3h: %w", o.waitTimeout, err)
	}

	kind := args[0]

	// identifier is optional to allow for commands like `zarf tools wait-for storageclass` without specifying a name.
	identifier := ""
	if len(args) > 1 {
		identifier = args[1]
	}

	condition := ""
	if len(args) > 2 {
		condition = args[2]
	}

	switch kind {
	case "http", "https", "tcp":
		return wait.ForNetwork(cmd.Context(), kind, identifier, condition, timeout)
	default:
		return wait.ForResource(cmd.Context(), kind, identifier, condition, o.waitNamespace, timeout)
	}
}

type waitForResourceOptions struct {
	timeout   string
	namespace string
}

func newWaitForResourceCommand() *cobra.Command {
	o := waitForResourceOptions{}
	cmd := &cobra.Command{
		Use:     "resource KIND NAME [CONDITION]",
		Short:   lang.CmdToolsWaitForResourceShort,
		Long:    lang.CmdToolsWaitForResourceLong,
		Example: lang.CmdToolsWaitForResourceExample,
		Args:    cobra.MinimumNArgs(2),
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
	identifier := args[1]

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
		Args:    cobra.MinimumNArgs(2),
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
