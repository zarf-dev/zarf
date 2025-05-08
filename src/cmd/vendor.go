// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"
	"strings"

	"slices"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config"
)

var vendorCmds = []string{
	"kubectl",
	"k",
	"syft",
	"sbom",
	"s",
	"k9s",
	"monitor",
	"m",
	"wait",
	"w",
	"crane",
	"registry",
	"r",
	"helm",
	"h",
	"yq",
}

// checkVendorOnlyFromArgs checks if the command being run is a vendor-only command
func checkVendorOnlyFromArgs() bool {
	// Check for "zarf tools|t <cmd>" where <cmd> is in the vendorCmd list
	return IsVendorCmd(os.Args, vendorCmds)
}

// checkVendorOnlyFromPath checks if the cobra command is a vendor-only command
func checkVendorOnlyFromPath(cmd *cobra.Command) bool {
	args := strings.Split(cmd.CommandPath(), " ")
	// Check for "zarf tools|t <cmd>" where <cmd> is in the vendorCmd list
	return IsVendorCmd(args, vendorCmds)
}

// IsVendorCmd checks if the command is a vendor command.
func IsVendorCmd(args []string, vendoredCmds []string) bool {
	if config.ActionsCommandZarfPrefix != "" {
		args = args[1:]
	}

	if len(args) > 2 {
		if args[1] == "tools" || args[1] == "t" {
			if slices.Contains(vendoredCmds, args[2]) {
				return true
			}
		}
	}

	return false
}
