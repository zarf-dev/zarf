// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cliflag "k8s.io/component-base/cli/flag"
	kubectl "k8s.io/kubectl/pkg/cmd"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// Updated command thanks to k0s
// https://github.com/k0sproject/k0s/blob/df88db5f317bb84dcda797ff6a68956bc2e49683/cmd/kubectl/kubectl.go#L59
func newKubectlCommand() *cobra.Command {
	kubectlCmd := kubectl.NewKubectlCommand(kubectl.KubectlOptions{
		IOStreams: genericiooptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	})

	kubectlCmd.Use = "kubectl"
	kubectlCmd.Aliases = []string{"k"}
	kubectlCmd = ReplaceCommandName("kubectl", "zarf tools kubectl", kubectlCmd)

	kubectlCmd.SetGlobalNormalizationFunc(cliflag.WordSepNormalizeFunc)
	kubectlCmd.SilenceErrors = true

	return kubectlCmd
}

// Pulled from deckhouse-cli, thank you.
// https://github.com/deckhouse/deckhouse-cli/blob/7e0c1e743b16c82134a062985dde161178bd45f6/cmd/commands/utils.go#L25
func ReplaceCommandName(from, to string, c *cobra.Command) *cobra.Command {
	c.Example = strings.ReplaceAll(c.Example, from, to)
	// Need some investigation about links
	// c.Long = strings.Replace(c.Long, from, to, -1)
	for _, sub := range c.Commands() {
		ReplaceCommandName(from, to, sub)
	}
	return c
}
