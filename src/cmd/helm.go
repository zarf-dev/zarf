// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	helmcmd "helm.sh/helm/v4/pkg/cmd"
	"helm.sh/helm/v4/pkg/kube"
)

func newHelmCommand(args []string) *cobra.Command {
	// Only pass arguments belonging to Helm to its initialization.
	var helmArgs []string
	if len(args) > 0 && args[0] == "helm" {
		helmArgs = args[1:]
	}

	cmd, err := helmcmd.NewRootCmd(os.Stdout, helmArgs, helmcmd.SetupLogging)
	if err != nil {
		logger.Default().Error("Helm command initialization", slog.Any("error", err))
	}

	original := cmd.PersistentPreRun
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		kube.ManagedFieldsManager = "helm"
		if original != nil {
			original(cmd, args)
		}
	}

	// Explicitly set cmd.Help() here so cobra still generates the docs page correctly
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	}

	return ReplaceCommandName("helm", "zarf tools helm", cmd)
}
