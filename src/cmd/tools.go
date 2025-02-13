// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
)

func newToolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tools",
		Aliases: []string{"t"},
		Short:   lang.CmdToolsShort,
	}

	v := getViper()

	cmd.AddCommand(newArchiverCommand())
	cmd.AddCommand(newRegistryCommand())
	cmd.AddCommand(newHelmCommand())
	cmd.AddCommand(newK9sCommand())
	cmd.AddCommand(newKubectlCommand())
	cmd.AddCommand(newSbomCommand())
	cmd.AddCommand(newWaitForCommand())
	cmd.AddCommand(newYQCommand())
	cmd.AddCommand(newGetCredsCommand())
	cmd.AddCommand(newUpdateCredsCommand(v))
	cmd.AddCommand(newClearCacheCommand())
	cmd.AddCommand(newDownloadInitCommand())
	cmd.AddCommand(newGenPKICommand())
	cmd.AddCommand(newGenKeyCommand())

	return cmd
}

func newToolsVersionCmd(name, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Args:  cobra.NoArgs,
		Short: lang.CmdToolsVersionShort,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Println(fmt.Sprintf("%s %s", name, version))
		},
	}
}
