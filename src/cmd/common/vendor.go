// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package common handles command configuration across all commands
package common

import (
	"os"
	"strings"

	"slices"

	"github.com/spf13/cobra"
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
	"helm",
	"h",
}

// CheckVendorOnlyFromArgs checks if the command being run is a vendor-only command
func CheckVendorOnlyFromArgs() bool {
	// Check for "zarf tools|t <cmd>" where <cmd> is in the vendorCmd list
	return IsVendorCmd(os.Args, vendorCmds)
}

// CheckVendorOnlyFromPath checks if the cobra command is a vendor-only command
func CheckVendorOnlyFromPath(cmd *cobra.Command) bool {
	args := strings.Split(cmd.CommandPath(), " ")
	// Check for "zarf tools|t <cmd>" where <cmd> is in the vendorCmd list
	return IsVendorCmd(args, vendorCmds)
}

// IsVendorCmd checks if the command is a vendor command.
func IsVendorCmd(args []string, vendoredCmds []string) bool {
	if len(args) > 2 {
		if args[1] == "tools" || args[1] == "t" {
			if slices.Contains(vendoredCmds, args[2]) {
				return true
			}
		}
	}

	return false
}
