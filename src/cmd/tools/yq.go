// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	yq "github.com/mikefarah/yq/v4/cmd"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
)

// NewYQCommand creates the `tools yq` sub-command and its nested children.
func NewYQCommand() *cobra.Command {
	cmd := yq.New()
	cmd.Example = lang.CmdToolsYqExample
	cmd.Use = "yq"
	for _, subCmd := range cmd.Commands() {
		if subCmd.Name() == "eval" {
			subCmd.Example = lang.CmdToolsYqEvalExample
		}
		if subCmd.Name() == "eval-all" {
			subCmd.Example = lang.CmdToolsYqEvalAllExample
		}
	}

	return cmd
}
