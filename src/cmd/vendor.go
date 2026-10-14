// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"io"
	"os"
	"slices"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
	for cmd.HasParent() {
		if cmd.Parent().Name() == "tools" {
			return slices.Contains(vendorCmds, cmd.Name())
		}
		cmd = cmd.Parent()
	}
	return false
}

// IsVendorCmd checks if the command is a vendor command.
func IsVendorCmd(args []string, vendoredCmds []string) bool {
	invocation, err := parseVendorInvocation(args, vendoredCmds)
	return err == nil && invocation != nil
}

type vendorInvocation struct {
	flags    []string
	args     []string
	toolArgs []string
}

// parseVendorInvocation finds the tool boundary using Zarf's flag definitions.
// It leaves arguments after the tool name untouched, including tool flags.
func parseVendorInvocation(args []string, vendoredCmds []string) (*vendorInvocation, error) {
	if len(args) == 0 {
		return nil, nil
	}
	args = args[1:]
	if config.ActionsCommandZarfPrefix != "" && len(args) > 0 {
		args = args[1:]
	}
	var completion string
	if len(args) > 0 && (args[0] == cobra.ShellCompRequestCmd || args[0] == cobra.ShellCompNoDescRequestCmd) {
		completion = args[0]
		args = args[1:]
	}

	var options rootOptions
	flags := newRootFlagSet(viper.New(), &options)
	flags.BoolP("help", "h", false, "help for zarf")
	flags.SetInterspersed(false)
	flags.SetOutput(io.Discard)
	// Discovery consumes flag values without applying or validating them. The
	// selected command's parser validates them before execution.
	discover := func(*pflag.Flag, string) error { return nil }
	if err := flags.ParseAll(args, discover); err != nil {
		return nil, err
	}
	remaining := flags.Args()
	if flags.ArgsLenAtDash() >= 0 || len(remaining) == 0 || (remaining[0] != "tools" && remaining[0] != "t") {
		return nil, nil
	}
	prefixFlags := args[:len(args)-len(remaining)]
	if err := flags.ParseAll(remaining[1:], discover); err != nil {
		return nil, err
	}
	toolArgs := flags.Args()
	if flags.ArgsLenAtDash() >= 0 || len(toolArgs) == 0 || !slices.Contains(vendoredCmds, toolArgs[0]) {
		return nil, nil
	}
	toolPrefixFlags := remaining[1 : len(remaining)-len(toolArgs)]
	invocation := &vendorInvocation{
		flags:    append(slices.Clone(prefixFlags), toolPrefixFlags...),
		args:     append([]string{remaining[0]}, toolArgs...),
		toolArgs: slices.Clone(toolArgs),
	}
	if completion != "" {
		invocation.args = append([]string{completion}, invocation.args...)
	}
	return invocation, nil
}
