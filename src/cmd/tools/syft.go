// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	syftCLI "github.com/anchore/syft/cmd/syft/cli"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
)

func init() {
	syftCmd, err := syftCLI.New()
	if err != nil {
		message.Fatal(err, lang.CmdToolsSbomErr)
	}
	syftCmd.Use = "sbom"
	syftCmd.Short = lang.CmdToolsSbomShort
	syftCmd.Aliases = []string{"s", "syft"}
	syftCmd.Example = ""

	for _, subCmd := range syftCmd.Commands() {
		subCmd.Example = ""
	}

	toolsCmd.AddCommand(syftCmd)
}
