// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"time"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/spf13/cobra"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	waitTimeout   string
	waitNamespace string
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

func init() {
	toolsCmd.AddCommand(waitForCmd)
	waitForCmd.Flags().StringVar(&waitTimeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
	waitForCmd.Flags().StringVarP(&waitNamespace, "namespace", "n", "", lang.CmdToolsWaitForFlagNamespace)
	waitForCmd.Flags().BoolVar(&message.NoProgress, "no-progress", false, lang.RootCmdFlagNoProgress)
}
