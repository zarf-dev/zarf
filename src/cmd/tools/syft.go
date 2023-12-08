// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"github.com/anchore/clio"
	syftCLI "github.com/anchore/syft/cmd/syft/cli"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
)

func init() {
	syftCmd := syftCLI.Command(clio.Identification{
		Name:    "zarf",
		Version: config.CLIVersion,
	})
	syftCmd.Use = "sbom"
	syftCmd.Short = lang.CmdToolsSbomShort
	syftCmd.Aliases = []string{"s", "syft"}
	syftCmd.Example = ""

	for _, subCmd := range syftCmd.Commands() {
		subCmd.Example = ""
	}

	toolsCmd.AddCommand(syftCmd)
}
