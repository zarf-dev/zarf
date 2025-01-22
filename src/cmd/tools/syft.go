// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"github.com/anchore/clio"
	syftCLI "github.com/anchore/syft/cmd/syft/cli"
	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"
)

// ldflags github.com/zarf-dev/zarf/src/cmd/tools.syftVersion=x.x.x
var syftVersion string

func newSbomCommand() *cobra.Command {
	cmd := syftCLI.Command(clio.Identification{
		Name:    "syft",
		Version: syftVersion,
	})
	cmd.Use = "sbom"
	cmd.Short = lang.CmdToolsSbomShort
	cmd.Aliases = []string{"s", "syft"}
	cmd.Example = ""

	for _, subCmd := range cmd.Commands() {
		subCmd.Example = ""
	}

	return cmd
}
