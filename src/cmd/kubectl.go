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

// code credit, k0sproject/k0s
// https://github.com/k0sproject/k0s/blob/df88db5f317bb84dcda797ff6a68956bc2e49683/cmd/kubectl/kubectl.go#L59
func newKubectlCommand() *cobra.Command {
	kubectlCmd := kubecmd.NewDefaultKubectlCommandWithArgs(kubecmd.KubectlOptions{
		IOStreams: genericiooptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
		// 2 is a magic number, but instead of sending `zarf tools kubectl get nodes` to kubectl
		// it sends just `kubectl get nodes` or `k get nodes`
		Arguments: os.Args[2:],
		PluginHandler: &kubecmd.DefaultPluginHandler{
			ValidPrefixes: plugin.ValidPluginFilenamePrefixes,
		},
	})

	kubectlCmd.Use = "kubectl"
	kubectlCmd.Aliases = []string{"k"}
	kubectlCmd.SilenceErrors = true

	patchPluginListSubcommand(kubectlCmd)

	return ReplaceCommandName("kubectl", "zarf tools kubectl", kubectlCmd)
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
