// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var vendorCmds = []string{
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
	"crane",
	"registry",
	"r",
}

var (
	// Viper instance used by the tools package (shared with cmd)
	v *viper.Viper
)

var toolsCmd = &cobra.Command{
	Use:     "tools",
	Aliases: []string{"t"},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.SkipLogFile = true
		exec.ExitOnInterrupt()
	},
	Short: lang.CmdToolsShort,
}

// Include adds the tools command to the root command.
func Include(rootCmd *cobra.Command, cmdViper *viper.Viper) {
	rootCmd.AddCommand(toolsCmd)
	v = cmdViper
}

// CheckVendorOnlyFromArgs checks if the command being run is a vendor-only command
func CheckVendorOnlyFromArgs() bool {
	// Check for "zarf tools|t <cmd>" where <cmd> is in the vendorCmd list
	return isVendorCmd(os.Args, vendorCmds)
}

// CheckVendorOnlyFromPath checks if the cobra command is a vendor-only command
func CheckVendorOnlyFromPath(cmd *cobra.Command) bool {
	args := strings.Split(cmd.CommandPath(), " ")
	// Check for "zarf tools|t <cmd>" where <cmd> is in the vendorCmd list
	return isVendorCmd(args, vendorCmds)
}

// isVendorCmd checks if the command is a vendor command.
func isVendorCmd(args []string, vendoredCmds []string) bool {
	if len(args) > 2 {
		if args[1] == "tools" || args[1] == "t" {
			if helpers.SliceContains(vendoredCmds, args[2]) {
				return true
			}
		}
	}

	return false
}
