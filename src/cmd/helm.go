// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/kube"
	helmcmd "helm.sh/helm/v4/pkg/cmd"
)

func newHelmCommand() *cobra.Command {
	// Truncate Helm's arguments so that it thinks its all alone
	helmArgs := []string{}
	if len(os.Args) > 2 {
		helmArgs = os.Args[3:]
	}
	// FIXME: what do I want this to be set to? Should it be set to the same thing during `zarf tools helm` as it is during Zarf operations
	kube.ManagedFieldsManager = "helm"

	cmd, err := helmcmd.NewRootCmd(os.Stdout, helmArgs, helmcmd.SetupLogging)
	if err != nil {
		slog.Warn("command failed", slog.Any("error", err))
		os.Exit(1)
	}

	return cmd
}
