// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/spf13/cobra"
)

var toolsCmd = &cobra.Command{
	Use:     "tools",
	Aliases: []string{"t"},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.SkipLogFile = true
	},
	Short: lang.CmdToolsShort,
}

// Include adds the tools command to the root command.
func Include(rootCmd *cobra.Command) {
	rootCmd.AddCommand(toolsCmd)
}

// CheckVendorOnly checks if the command is being run as a vendor-only command
func CheckVendorOnly() bool {
	vendorCmd := []string{
		"kubectl",
		"k",
		"syft",
		"sbom",
		"s",
		"k9s",
		"monitor",
		"wait-for",
		"wait",
		"w",
	}

	// Check for "zarf tools|t <cmd>" where <cmd> is in the vendorCmd list
	return isVendorCmd(vendorCmd)
}

// isVendorCmd checks if the command is a vendor command.
func isVendorCmd(cmd []string) bool {
	a := os.Args
	if len(a) > 2 {
		if a[1] == "tools" || a[1] == "t" {
			if utils.SliceContains(cmd, a[2]) {
				return true
			}
		}
	}

	return false
}
