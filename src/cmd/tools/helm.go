// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"os"

	"github.com/zarf-dev/zarf/src/pkg/message"

	"github.com/zarf-dev/zarf/src/cmd/tools/helm"
	"github.com/zarf-dev/zarf/src/config/lang"
	"helm.sh/helm/v3/pkg/action"
)

// ldflags github.com/zarf-dev/zarf/src/cmd/tools.helmVersion=x.x.x
var helmVersion string

func init() {
	actionConfig := new(action.Configuration)

	// Truncate Helm's arguments so that it thinks its all alone
	helmArgs := []string{}
	if len(os.Args) > 2 {
		helmArgs = os.Args[3:]
	}
	// The inclusion of Helm in this manner should be changed once https://github.com/helm/helm/pull/12725 is merged
	helmCmd, err := helm.NewRootCmd(actionConfig, os.Stdout, helmArgs)
	if err != nil {
		message.Debug("Failed to initialize helm command", "error", err)
	}
	helmCmd.Short = lang.CmdToolsHelmShort
	helmCmd.Long = lang.CmdToolsHelmLong
	helmCmd.AddCommand(newVersionCmd("helm", helmVersion))

	toolsCmd.AddCommand(helmCmd)
}
