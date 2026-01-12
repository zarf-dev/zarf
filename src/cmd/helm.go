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

func newHelmCommand() *cobra.Command {
	// Truncate Helm's arguments so that it thinks its all alone
	helmArgs := []string{}
	if len(os.Args) > 2 {
		helmArgs = os.Args[3:]
	}

	kube.ManagedFieldsManager = "helm"

	cmd, err := helmcmd.NewRootCmd(os.Stdout, helmArgs, helmcmd.SetupLogging)
	if err != nil {
		logger.Default().Error("Helm command initialization", slog.Any("error", err))
		os.Exit(1)
	}

	return cmd
}
