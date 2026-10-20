// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	helmcmd "helm.sh/helm/v4/pkg/cmd"
	"helm.sh/helm/v4/pkg/kube"
)

func newHelmCommand(args []string) (*cobra.Command, error) {
	cmd, err := helmcmd.NewRootCmd(os.Stdout, args, helmcmd.SetupLogging)
	if err != nil {
		return nil, err
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

	return ReplaceCommandName("helm", "zarf tools helm", cmd), nil
}
