// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	helmcmd "helm.sh/helm/v4/pkg/cmd"
	"helm.sh/helm/v4/pkg/kube"
)

func newHelmCommand() *cobra.Command {
	// Truncate Helm's arguments so that it thinks its all alone
	helmArgs := []string{}
	if len(os.Args) > 2 {
		helmArgs = os.Args[3:]
	}
	// FIXME: what do I want this to be set to? Should it be set to the same thing during `zarf tools helm` as it is during Zarf operations
	// My gut tells me this should be set to helm for helm operations and zarf for Zarf operations? I do want to check in what scenarios this matters
	// - if I install a chart with Zarf, then edit with kubectl, will it matter?
	kube.ManagedFieldsManager = "helm"

	cmd, err := helmcmd.NewRootCmd(os.Stdout, helmArgs, helmcmd.SetupLogging)
	if err != nil {
		slog.Warn("command failed", slog.Any("error", err))
		os.Exit(1)
	}

	return cmd
}
