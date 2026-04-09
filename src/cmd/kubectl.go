// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubecmd "k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/plugin"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const (
	kubeShort = "k"
	kubeLong  = "kubectl"
)

func newKubectlCommand() *cobra.Command {
	var kubectlCmd *cobra.Command

	argOffset := -1
	for i, arg := range os.Args {
		if arg == "tools" || arg == "t" {
			if i+1 < len(os.Args) && (os.Args[i+1] == kubeLong || os.Args[i+1] == kubeShort) {
				argOffset = i + 1
				break
			}
		}
	}

	opts := kubecmd.KubectlOptions{
		IOStreams: genericiooptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}

	if argOffset > -1 {
		opts.Arguments = os.Args[argOffset:]
		opts.PluginHandler = kubecmd.NewDefaultPluginHandler(plugin.ValidPluginFilenamePrefixes)
	}

	kubectlCmd = kubecmd.NewDefaultKubectlCommandWithArgs(opts)
	kubectlCmd.Use = kubeLong
	kubectlCmd.Aliases = []string{kubeShort}
	kubectlCmd.SilenceErrors = true

	patchPluginListSubcommand(kubectlCmd)

	return ReplaceCommandName(kubeLong, "zarf tools kubectl", kubectlCmd)
}

// patchPluginListSubcommand patches kubectl's "plugin list" command in a way
// that it will look at the kubectl command, not at the zarf command for
// detecting shadowed commands. Kubectl's current implementation of that command
// looks at the root command for detecting collisions. In case of zarf, which
// embeds kubectl as a subcommand rather than at the top level, this means that,
// instead of looking at kubectl command itself, the logic would look at zarf and
// produce the wrong output.
// code credit, k0sproject/k0s
// https://github.com/k0sproject/k0s/blob/df88db5f317bb84dcda797ff6a68956bc2e49683/cmd/kubectl/kubectl.go#L202
func patchPluginListSubcommand(kubectlCmd *cobra.Command) {
	cmd, _, err := kubectlCmd.Find([]string{"plugin", "list"})
	cmdutil.CheckErr(err)

	originalRun := cmd.Run
	cmd.Run = func(_ *cobra.Command, args []string) {
		root := kubecmd.NewKubectlCommand(kubecmd.KubectlOptions{})
		root.Use = strings.ReplaceAll(kubectlCmd.CommandPath(), " ", "-")
		originalRun(root, args)
	}
}
