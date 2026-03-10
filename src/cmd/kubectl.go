// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubecmd "k8s.io/kubectl/pkg/cmd"
	"k8s.io/kubectl/pkg/cmd/plugin"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// code credit, k0sproject/k0s
// https://github.com/k0sproject/k0s/blob/df88db5f317bb84dcda797ff6a68956bc2e49683/cmd/kubectl/kubectl.go#L59
func newKubectlCommand() *cobra.Command {
	kubectlCmd := kubecmd.NewKubectlCommand(kubecmd.KubectlOptions{
		IOStreams: genericiooptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	})

	kubectlCmd.Use = "kubectl"
	kubectlCmd.Aliases = []string{"k"}
	kubectlCmd.SilenceErrors = true

	hookKubectlPluginHandler(kubectlCmd)
	patchPluginListSubcommand(kubectlCmd)

	return ReplaceCommandName("kubectl", "zarf tools kubectl", kubectlCmd)
}

// hookKubectlPluginHandler patches the kubectl command in a way that it will
// execute kubectl's plugin handler before actually executing the command.
// code credit, k0sproject/k0s
// https://github.com/k0sproject/k0s/blob/df88db5f317bb84dcda797ff6a68956bc2e49683/cmd/kubectl/kubectl.go#L83
func hookKubectlPluginHandler(kubectlCmd *cobra.Command) {
	originalFlagErrFunc := kubectlCmd.FlagErrorFunc()
	kubectlCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		handleKubectlPlugins(kubectlCmd)
		return originalFlagErrFunc(cmd, err)
	})

	originalPreRunE := kubectlCmd.PersistentPreRunE
	kubectlCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		handleKubectlPlugins(kubectlCmd)

		if cmd == kubectlCmd {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
		}

		return originalPreRunE(cmd, args)
	}
}

// handleKubectlPlugins calls kubectl's plugin handler and execs the plugin
// without returning if there's any plugin available that handles the given
// command line arguments. Will simply return otherwise.
// code credit, k0sproject/k0s
// https://github.com/k0sproject/k0s/blob/df88db5f317bb84dcda797ff6a68956bc2e49683/cmd/kubectl/kubectl.go#L134
func handleKubectlPlugins(kubectlCmd *cobra.Command) {
	// Check how the kubectl command has been called on the command line.
	calledAs := kubectlCmd.CalledAs()
	if calledAs == "" {
		return
	}

	// Find the first occurrence of the kubectl command on the command line.
	argOffset := slices.Index(os.Args, calledAs)
	if argOffset < 0 {
		return
	}

	_ = kubecmd.NewDefaultKubectlCommandWithArgs(kubecmd.KubectlOptions{
		IOStreams: genericclioptions.IOStreams{
			In:     kubectlCmd.InOrStdin(),
			Out:    kubectlCmd.OutOrStdout(),
			ErrOut: kubectlCmd.ErrOrStderr(),
		},
		Arguments: os.Args[argOffset:],
		PluginHandler: &kubecmd.DefaultPluginHandler{
			ValidPrefixes: plugin.ValidPluginFilenamePrefixes,
		},
	})
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
	cmd.Run = func(cmd *cobra.Command, args []string) {
		root := kubecmd.NewKubectlCommand(kubecmd.KubectlOptions{})
		root.Use = strings.ReplaceAll(kubectlCmd.CommandPath(), " ", "-")
		originalRun(root, args)
	}
}
