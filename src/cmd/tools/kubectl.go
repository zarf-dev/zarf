// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tools contains the CLI commands for Zarf.
package tools

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/zarf-dev/zarf/src/cmd/common"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	kubeCLI "k8s.io/component-base/cli"
	kubeCmd "k8s.io/kubectl/pkg/cmd"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// NewKubectlCommand creates the `tools kubectl` sub-command.
func NewKubectlCommand() *cobra.Command {
	// Kubectl stub command.
	cmd := &cobra.Command{
		Short: lang.CmdToolsKubectlDocs,
		Run:   func(_ *cobra.Command, _ []string) {},
	}

	// Only load this command if it is being called directly.
	if common.IsVendorCmd(os.Args, []string{"kubectl", "k"}) {
		// Add the kubectl command to the tools command.
		cmd = kubeCmd.NewDefaultKubectlCommand()

		if err := kubeCLI.RunNoErrOutput(cmd); err != nil {
			// @todo(jeff-mccoy) - Kubectl gets mad about being a subcommand.
			message.Debug(err)
			logger.Default().Debug(err.Error())
		}
	}

	cmd.Use = "kubectl"
	cmd.Aliases = []string{"k"}

	return cmd
}
