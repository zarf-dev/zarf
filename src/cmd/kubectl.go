// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectl "k8s.io/kubectl/pkg/cmd"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// code credit, k0sproject/k0s
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
	kubectlCmd.SilenceErrors = true

	return ReplaceCommandName("kubectl", "zarf tools kubectl", kubectlCmd)
}
