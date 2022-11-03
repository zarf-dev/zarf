// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for zarf
package cmd

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		skipLogFile = true
		cliSetup()
	},
	Short: "Displays the version of the Zarf binary",
	Long:  "Displays the version of the Zarf release that the Zarf binary was built from.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.CLIVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
