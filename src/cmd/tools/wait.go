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
	waitType      string
)

var waitForCmd = &cobra.Command{
	Use:     "wait-for { KIND | PROTOCOL } { NAME | SELECTOR | URI } { CONDITION | HTTP_CODE }",
	Aliases: []string{"w", "wait"},
	Short:   lang.CmdToolsWaitForShort,
	Long:    lang.CmdToolsWaitForLong,
	Example: lang.CmdToolsWaitForExample,
	Args:    cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse the timeout string
		timeout, err := time.ParseDuration(waitTimeout)
		if err != nil {
			message.Fatalf(err, lang.CmdToolsWaitForErrTimeoutString, waitTimeout)
		}

		// Parse the kind type and identifier.
		kind, identifier := args[0], args[1]

		// Condition is optional, default to "exists".
		condition := ""
		if len(args) > 2 {
			condition = args[2]
		}

		// Execute the wait command.
		utils.ExecuteWait(waitTimeout, waitNamespace, condition, kind, identifier, timeout)
	},
}

func init() {
	toolsCmd.AddCommand(waitForCmd)
	waitForCmd.Flags().StringVar(&waitTimeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
	waitForCmd.Flags().StringVarP(&waitNamespace, "namespace", "n", "", lang.CmdToolsWaitForFlagNamespace)
}
