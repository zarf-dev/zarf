// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	yq "github.com/mikefarah/yq/v4/cmd"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
)

func newYQCommand() *cobra.Command {
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
