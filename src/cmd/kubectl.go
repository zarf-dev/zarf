// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubecmd "k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/plugin"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	kubeShort = "k"
	kubeLong  = "kubectl"
)

func newKubectlCommand(args []string) *cobra.Command {
	opts := kubecmd.KubectlOptions{
		IOStreams: genericiooptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}

	if args != nil {
		opts.Arguments = append([]string{kubeLong}, args...)
		opts.PluginHandler = kubecmd.NewDefaultPluginHandler(plugin.ValidPluginFilenamePrefixes)
	}

	kubectlCmd := kubecmd.NewDefaultKubectlCommandWithArgs(opts)
	kubectlCmd.Use = kubeLong
	kubectlCmd.Aliases = []string{kubeShort}
	kubectlCmd.SilenceErrors = true

	return ReplaceCommandName(kubeLong, "zarf tools kubectl", kubectlCmd)
}
