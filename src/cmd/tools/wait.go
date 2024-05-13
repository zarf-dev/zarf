// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"context"
	"time"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/spf13/cobra"

	// Import to initialize client auth plugins.
	"k8s.io/apimachinery/pkg/runtime/schema"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	waitTimeout   string
	waitNamespace string

	waitResourceTimeout    string
	waitResourceNamespace  string
	waitResourceKubeconfig string
)

var waitForCmd = &cobra.Command{
	Use:     "wait-for { KIND | PROTOCOL } { NAME | SELECTOR | URI } { CONDITION | HTTP_CODE }",
	Aliases: []string{"w", "wait"},
	Short:   lang.CmdToolsWaitForShort,
	Long:    lang.CmdToolsWaitForLong,
	Example: lang.CmdToolsWaitForExample,
	Args:    cobra.MinimumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		// Parse the timeout string
		timeout, err := time.ParseDuration(waitTimeout)
		if err != nil {
			message.Fatalf(err, lang.CmdToolsWaitForErrTimeoutString, waitTimeout)
		}

		kind := args[0]

		// identifier is optional to allow for commands like `zarf tools wait-for storageclass` without specifying a name.
		identifier := ""
		if len(args) > 1 {
			identifier = args[1]
		}

		// Condition is optional, default to "exists".
		condition := ""
		if len(args) > 2 {
			condition = args[2]
		}

		// Execute the wait command.
		if err := utils.ExecuteWait(waitTimeout, waitNamespace, condition, kind, identifier, timeout); err != nil {
			message.Fatal(err, err.Error())
		}
	},
}

var waitForResourceCmd = &cobra.Command{
	Use:     "wait-for-resource { kind.group | kind.version.group } { NAME }",
	Short:   lang.CmdToolsWaitForShort,
	Long:    lang.CmdToolsWaitForLong,
	Example: lang.CmdToolsWaitForExample,
	Args:    cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		timeout, err := time.ParseDuration(waitResourceTimeout)
		if err != nil {
			message.Fatalf(err, lang.CmdToolsWaitForErrTimeoutString, waitResourceTimeout)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		// Accept passing version even if we ignore it.
		gvk, gk := schema.ParseKindArg(args[0])
		if gvk != nil {
			gk = gvk.GroupKind()
		}
		err = utils.ExecuteWaitResource(ctx, gk, waitResourceNamespace, args[1])
		if err != nil {
			message.Fatal(err, err.Error())
		}
	},
}

func init() {
	toolsCmd.AddCommand(waitForCmd)
	waitForCmd.Flags().StringVar(&waitTimeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
	waitForCmd.Flags().StringVarP(&waitNamespace, "namespace", "n", "", lang.CmdToolsWaitForFlagNamespace)
	waitForCmd.Flags().BoolVar(&message.NoProgress, "no-progress", false, lang.RootCmdFlagNoProgress)

	toolsCmd.AddCommand(waitForResourceCmd)
	waitForResourceCmd.Flags().StringVar(&waitResourceTimeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
	waitForResourceCmd.Flags().StringVarP(&waitResourceKubeconfig, "kubeconfig", "f", "", lang.CmdToolsWaitForFlagNamespace)
	waitForResourceCmd.Flags().StringVarP(&waitResourceNamespace, "namespace", "n", "", lang.CmdToolsWaitForFlagNamespace)
	waitForResourceCmd.Flags().BoolVar(&message.NoProgress, "no-progress", false, lang.RootCmdFlagNoProgress)
}
