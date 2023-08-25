// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"os"

	"github.com/defenseunicorns/zarf/src/cmd/tools/helm"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"helm.sh/helm/v3/pkg/action"
)

func init() {
	actionConfig := new(action.Configuration)

	// The inclusion of Helm in this manner should be reconsidered once https://github.com/helm/helm/issues/12122 is resolved
	helmCmd, _ := helm.NewRootCmd(actionConfig, os.Stdout, os.Args[3:])
	helmCmd.Short = lang.CmdToolsHelmShort
	helmCmd.Long = lang.CmdToolsHelmLong

	toolsCmd.AddCommand(helmCmd)
}
