// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	helmcmd "helm.sh/helm/v3/pkg/cmd"
)

func init() {
	actionConfig := new(action.Configuration)
	settings := cli.New()

	// Since helm needs args passed into it, check if we are processing things on a command with fewer args
	if len(os.Args) < 3 {
		return
	}

	// The inclusion of Helm in this manner should be reconsidered once https://github.com/helm/helm/issues/12122 is resolved
	cmd, _ := helmcmd.NewRootCmd(actionConfig, settings, os.Stdout, os.Args[3:])
	// cmd.Short = lang.CmdToolsHelmShort
	// cmd.Long = lang.CmdToolsHelmLong

	toolsCmd.AddCommand(cmd)
}
