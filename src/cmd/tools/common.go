// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/cmd/common"
	"github.com/zarf-dev/zarf/src/config/lang"
)

// NewToolsCommand creates the `tools` sub-command and its nested children.
func NewToolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tools",
		Aliases: []string{"t"},
		Short:   lang.CmdToolsShort,
	}

	v := common.GetViper()

	cmd.AddCommand(NewArchiverCommand())
	cmd.AddCommand(NewRegistryCommand())
	cmd.AddCommand(NewHelmCommand())
	cmd.AddCommand(NewK9sCommand())
	cmd.AddCommand(NewKubectlCommand())
	cmd.AddCommand(NewSbomCommand())
	cmd.AddCommand(NewWaitForCommand())
	cmd.AddCommand(NewYQCommand())
	cmd.AddCommand(NewGetCredsCommand())
	cmd.AddCommand(NewUpdateCredsCommand(v))
	cmd.AddCommand(NewClearCacheCommand())
	cmd.AddCommand(NewDownloadInitCommand())
	cmd.AddCommand(NewGenPKICommand())
	cmd.AddCommand(NewGenKeyCommand())

	return cmd
}

// newVersionCmd is a generic version command for tools
func newVersionCmd(name, version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Args:  cobra.NoArgs,
		Short: lang.CmdToolsVersionShort,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Println(fmt.Sprintf("%s %s", name, version))
		},
	}
}
