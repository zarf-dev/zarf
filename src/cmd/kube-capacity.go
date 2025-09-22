// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/config/lang"

	kubeCapacity "github.com/robscott/kube-capacity/pkg/cmd"

	// This allows for go linkname to be used in this file.  Go linkname is used so that we can pull the CLI flags from kube-capacity and generate proper docs for the vendored tool.
	_ "unsafe"
)

//go:linkname kubeCapacityRootCmd github.com/robscott/kube-capacity/pkg/cmd.rootCmd
var kubeCapacityRootCmd *cobra.Command

func newKubCapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capacity",
		Short: lang.CmdToolsCapacityShot,
		RunE: func(_ *cobra.Command, _ []string) error {
			os.Args = []string{os.Args[0]}
			kubeCapacity.Execute()
			return nil
		},
	}

	cmd.Flags().AddFlagSet(kubeCapacityRootCmd.Flags())

	return cmd
}
