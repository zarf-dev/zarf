// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"github.com/defenseunicorns/zarf/src/config/lang"
	yq "github.com/mikefarah/yq/v4/cmd"

)

func init() {

	yqCmd := yq.New()
	yqCmd.Example = lang.CmdToolsYqExample
	yqCmd.Use = "yq"
	for _, subCmd := range yqCmd.Commands() {
		if subCmd.Name() == "eval" {		
			subCmd.Example = lang.CmdToolsYqEvalExample
		}
		if subCmd.Name() == "eval-all" {	
			subCmd.Example = lang.CmdToolsYqEvalAllExample
		}
	}

	toolsCmd.AddCommand(yqCmd)
}
