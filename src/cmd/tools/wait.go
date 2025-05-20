// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/utils"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// WaitForOptions holds the command-line options for 'tools registry' sub-command.
type WaitForOptions struct {
	waitTimeout   string
	waitNamespace string
}

// NewWaitForCommand creates the `tools wait-for` sub-command.
func NewWaitForCommand() *cobra.Command {
	o := WaitForOptions{}
	cmd := &cobra.Command{
		Use:     "wait-for { KIND | PROTOCOL } { NAME | SELECTOR | URI } { CONDITION | HTTP_CODE }",
		Aliases: []string{"w", "wait"},
		Short:   lang.CmdToolsWaitForShort,
		Long:    lang.CmdToolsWaitForLong,
		Example: lang.CmdToolsWaitForExample,
		Args:    cobra.MinimumNArgs(1),
		RunE:    o.Run,
	}

	cmd.Flags().StringVar(&o.waitTimeout, "timeout", "5m", lang.CmdToolsWaitForFlagTimeout)
	cmd.Flags().StringVarP(&o.waitNamespace, "namespace", "n", "", lang.CmdToolsWaitForFlagNamespace)
	cmd.Flags().BoolVar(&message.NoProgress, "no-progress", false, lang.RootCmdFlagNoProgress)

	return cmd
}

// Run performs the execution of 'tools wait-for' sub-command.
func (o *WaitForOptions) Run(_ *cobra.Command, args []string) error {
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

	// Condition is optional, default to "exists".
	condition := ""
	if len(args) > 2 {
		condition = args[2]
	}

	// Execute the wait command.
	return utils.ExecuteWait(o.waitTimeout, o.waitNamespace, condition, kind, identifier, timeout)
}
