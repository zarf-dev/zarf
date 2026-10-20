// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zarf-dev/zarf/src/config/lang"
)

type embeddedTool struct {
	name       string
	aliases    []string
	short      string
	newCommand func([]string) (*cobra.Command, error)
}

var embeddedTools = []embeddedTool{
	{name: "helm", aliases: []string{"h"}, short: "The Helm package manager for Kubernetes.", newCommand: newHelmCommand},
	{name: "kubectl", aliases: []string{"k"}, short: "kubectl controls the Kubernetes cluster manager", newCommand: func(args []string) (*cobra.Command, error) {
		return newKubectlCommand(args), nil
	}},
	{name: "sbom", aliases: []string{"s", "syft"}, short: lang.CmdToolsSbomShort, newCommand: func([]string) (*cobra.Command, error) {
		return newSbomCommand(), nil
	}},
	{name: "monitor", aliases: []string{"m", "k9s"}, short: lang.CmdToolsMonitorShort, newCommand: func([]string) (*cobra.Command, error) {
		return newK9sCommand(), nil
	}},
	{name: "yq", short: "yq is a lightweight and portable command-line data file processor.", newCommand: func([]string) (*cobra.Command, error) {
		return newYQCommand(), nil
	}},
}

func findEmbeddedTool(name string) (embeddedTool, bool) {
	for _, tool := range embeddedTools {
		if tool.name == name || slices.Contains(tool.aliases, name) {
			return tool, true
		}
	}
	return embeddedTool{}, false
}

// Route tool execution, help, and completion before initializing Zarf configuration.
func embeddedToolInvocation(args []string) (embeddedTool, []string, bool) {
	var request []string
	if len(args) > 0 && (args[0] == cobra.ShellCompRequestCmd || args[0] == cobra.ShellCompNoDescRequestCmd) {
		request, args = append(request, args[0]), args[1:]
	}
	if len(args) > 0 && args[0] == "help" {
		request, args = append(request, args[0]), args[1:]
	}
	if len(args) < 2 || (args[0] != "tools" && args[0] != "t") {
		return embeddedTool{}, nil, false
	}
	tool, ok := findEmbeddedTool(args[1])
	args = append(request, args[2:]...)
	return tool, args, ok
}

func (tool embeddedTool) execute(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	// An empty, non-nil slice distinguishes execution from documentation construction.
	args = append([]string{}, args...)
	cmd, err := tool.newCommand(args)
	if err != nil {
		fmt.Fprintln(errOut, "Error:", err)
		return err
	}
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	cmd.Annotations[cobra.CommandDisplayNameAnnotation] = "zarf tools " + tool.name
	cmd.SetArgs(args)
	cmd.SetIn(in)
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	_, err = cmd.ExecuteContextC(ctx)
	if err != nil {
		cmd.PrintErrln(cmd.ErrPrefix(), err)
	}
	return err
}

// Keep routing entries in the Zarf tree; the tool's flags and hooks belong to its own root.
func (tool embeddedTool) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:                tool.name,
		Aliases:            tool.aliases,
		Short:              tool.short,
		DisableFlagParsing: true,
		PersistentPreRunE:  func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			var zarfFlag string
			cmd.VisitParents(func(parent *cobra.Command) {
				parent.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
					if flag.Changed {
						zarfFlag = flag.Name
					}
				})
			})
			if zarfFlag != "" {
				return fmt.Errorf("zarf flag --%s cannot be used with embedded tool %q; pass tool options after 'zarf tools %s'", zarfFlag, tool.name, tool.name)
			}
			return tool.execute(cmd.Context(), args, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
		ValidArgsFunction: tool.complete,
	}
	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		if err := tool.execute(cmd.Context(), []string{"--help"}, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
			cmd.PrintErrln(err)
		}
	})
	return cmd
}

func (tool embeddedTool) complete(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	var out bytes.Buffer
	request := append([]string{cobra.ShellCompRequestCmd}, args...)
	request = append(request, toComplete)
	if err := tool.execute(cmd.Context(), request, cmd.InOrStdin(), &out, cmd.ErrOrStderr()); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	directive, err := strconv.Atoi(strings.TrimPrefix(lines[len(lines)-1], ":"))
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return lines[:len(lines)-1], cobra.ShellCompDirective(directive)
}

// Cobra's default help stops at routing entries, so delegate nested tool topics explicitly.
func setupEmbeddedToolHelp(root *cobra.Command) {
	root.InitDefaultHelpCmd()
	for _, help := range root.Commands() {
		if help.Name() != "help" {
			continue
		}
		defaultRun, defaultComplete := help.Run, help.ValidArgsFunction
		help.RunE = func(cmd *cobra.Command, args []string) error {
			if tool, toolArgs, ok := embeddedToolInvocation(append([]string{"help"}, args...)); ok {
				return tool.execute(cmd.Context(), toolArgs, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			}
			defaultRun(cmd, args)
			return nil
		}
		help.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
			if tool, toolArgs, ok := embeddedToolInvocation(append([]string{"help"}, args...)); ok {
				return tool.complete(cmd, toolArgs, toComplete)
			}
			return defaultComplete(cmd, args, toComplete)
		}
	}
}
