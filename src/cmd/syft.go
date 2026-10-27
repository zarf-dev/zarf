// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"github.com/anchore/clio"
	syftCLI "github.com/anchore/syft/cmd/syft/cli"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
)

// ldflags github.com/zarf-dev/zarf/src/cmd.syftVersion=x.x.x
var syftVersion string

func newSbomCommand() *cobra.Command {
	cmd := syftCLI.Command(clio.Identification{
		Name:    "syft",
		Version: syftVersion,
	})
	cmd.Use = "sbom"
	cmd.Short = lang.CmdToolsSbomShort
	cmd.Aliases = []string{"s", "syft"}

	configureSbomConfigCommand(cmd)

	return ReplaceCommandName("syft", "zarf tools sbom", cmd)
}

func configureSbomConfigCommand(sbomCmd *cobra.Command) {
	configCmd, _, err := sbomCmd.Find([]string{"config"})
	if err != nil || configCmd == nil || configCmd.RunE == nil {
		return
	}

	runE := configCmd.RunE
	configCmd.RunE = func(cmd *cobra.Command, args []string) error {
		toolsCmd := sbomCmd.Parent()
		if toolsCmd == nil {
			return runE(cmd, args)
		}

		yqCmd, _, err := toolsCmd.Find([]string{"yq"})
		if err != nil || yqCmd == nil {
			return runE(cmd, args)
		}

		// Fangs walks every command flag while producing the Syft config summary. yq's
		// value-backed unwrapScalar flag causes Fangs to panic during that traversal.
		toolsCmd.RemoveCommand(yqCmd)
		defer toolsCmd.AddCommand(yqCmd)

		return runE(cmd, args)
	}
}
