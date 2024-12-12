// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
	"github.com/zarf-dev/zarf/src/cmd/common"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/agent"
	"github.com/zarf-dev/zarf/src/internal/gitea"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// NewInternalCommand creates the `internal` sub-command and its nested children.
func NewInternalCommand(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "internal",
		Hidden: true,
		Short:  lang.CmdInternalShort,
	}

	cmd.AddCommand(NewInternalAgentCommand())
	cmd.AddCommand(NewInternalHTTPProxyCommand())
	cmd.AddCommand(NewInternalGenCliDocsCommand(rootCmd))
	cmd.AddCommand(NewInternalCreateReadOnlyGiteaUserCommand())
	cmd.AddCommand(NewInternalCreateArtifactRegistryTokenCommand())
	cmd.AddCommand(NewInternalUpdateGiteaPVCCommand())
	cmd.AddCommand(NewInternalIsValidHostnameCommand())
	cmd.AddCommand(NewInternalCrc32Command())

	return cmd
}

// InternalAgentOptions holds the command-line options for 'internal agent' sub-command.
type InternalAgentOptions struct{}

// NewInternalAgentCommand creates the `internal agent` sub-command.
func NewInternalAgentCommand() *cobra.Command {
	o := &InternalAgentOptions{}

	cmd := &cobra.Command{
		Use:   "agent",
		Short: lang.CmdInternalAgentShort,
		Long:  lang.CmdInternalAgentLong,
		RunE:  o.Run,
	}

	return cmd
}

// Run performs the execution of 'internal agent' sub-command.
func (o *InternalAgentOptions) Run(cmd *cobra.Command, _ []string) error {
	cluster, err := cluster.NewCluster()
	if err != nil {
		return err
	}
	return agent.StartWebhook(cmd.Context(), cluster)
}

// InternalHTTPProxyOptions holds the command-line options for 'internal http-proxy' sub-command.
type InternalHTTPProxyOptions struct{}

// NewInternalHTTPProxyCommand creates the `internal http-proxy` sub-command.
func NewInternalHTTPProxyCommand() *cobra.Command {
	o := &InternalHTTPProxyOptions{}

	cmd := &cobra.Command{
		Use:   "http-proxy",
		Short: lang.CmdInternalProxyShort,
		Long:  lang.CmdInternalProxyLong,
		RunE:  o.Run,
	}

	return cmd
}

// Run performs the execution of 'internal http-proxy' sub-command.
func (o *InternalHTTPProxyOptions) Run(cmd *cobra.Command, _ []string) error {
	cluster, err := cluster.NewCluster()
	if err != nil {
		return err
	}
	return agent.StartHTTPProxy(cmd.Context(), cluster)
}

// InternalGenCliDocsOptions holds the command-line options for 'internal gen-cli-docs' sub-command.
type InternalGenCliDocsOptions struct {
	rootCmd *cobra.Command
}

// NewInternalGenCliDocsCommand creates the `internal gen-cli-docs` sub-command.
func NewInternalGenCliDocsCommand(root *cobra.Command) *cobra.Command {
	// TODO(soltysh): ideally this should be replace with cmd.Root() call from cobra
	o := &InternalGenCliDocsOptions{
		rootCmd: root,
	}

	cmd := &cobra.Command{
		Use:   "gen-cli-docs",
		Short: lang.CmdInternalGenerateCliDocsShort,
		RunE:  o.Run,
	}

	return cmd
}

// Run performs the execution of 'internal gen-cli-docs' sub-command.
func (o *InternalGenCliDocsOptions) Run(_ *cobra.Command, _ []string) error {
	// Don't include the datestamp in the output
	o.rootCmd.DisableAutoGenTag = true

	resetStringFlags := func(cmd *cobra.Command) {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if flag.Value.Type() == "string" {
				flag.DefValue = ""
			}
		})
	}

	for _, cmd := range o.rootCmd.Commands() {
		if cmd.Use == "tools" {
			for _, toolCmd := range cmd.Commands() {
				// If the command is a vendored command, add a dummy flag to hide root flags from the docs
				if common.CheckVendorOnlyFromPath(toolCmd) {
					addHiddenDummyFlag(toolCmd, "log-level")
					addHiddenDummyFlag(toolCmd, "log-format")
					addHiddenDummyFlag(toolCmd, "architecture")
					addHiddenDummyFlag(toolCmd, "no-log-file")
					addHiddenDummyFlag(toolCmd, "no-progress")
					addHiddenDummyFlag(toolCmd, "zarf-cache")
					addHiddenDummyFlag(toolCmd, "tmpdir")
					addHiddenDummyFlag(toolCmd, "insecure")
					addHiddenDummyFlag(toolCmd, "no-color")
				}

				// Remove the default values from all of the helm commands during the CLI command doc generation
				if toolCmd.Use == "helm" || toolCmd.Use == "sbom" {
					toolCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
						if flag.Value.Type() == "string" {
							flag.DefValue = ""
						}
					})
					resetStringFlags(toolCmd)
					for _, subCmd := range toolCmd.Commands() {
						resetStringFlags(subCmd)
						for _, helmSubCmd := range subCmd.Commands() {
							resetStringFlags(helmSubCmd)
						}
					}
				}

				if toolCmd.Use == "monitor" {
					resetStringFlags(toolCmd)
				}

				if toolCmd.Use == "yq" {
					for _, subCmd := range toolCmd.Commands() {
						if subCmd.Name() == "shell-completion" {
							subCmd.Hidden = true
						}
					}
				}
			}
		}
	}

	if err := os.RemoveAll("./site/src/content/docs/commands"); err != nil {
		return err
	}
	if err := os.Mkdir("./site/src/content/docs/commands", 0775); err != nil {
		return err
	}

	var prependTitle = func(s string) string {
		fmt.Println(s)

		name := filepath.Base(s)

		// strip .md extension
		name = name[:len(name)-3]

		// replace _ with space
		title := strings.Replace(name, "_", " ", -1)

		return fmt.Sprintf(`---
title: %s
description: Zarf CLI command reference for <code>%s</code>.
tableOfContents: false
---

<!-- Page generated by Zarf; DO NOT EDIT -->

`, title, title)
	}

	var linkHandler = func(link string) string {
		return "/commands/" + link[:len(link)-3] + "/"
	}

	return doc.GenMarkdownTreeCustom(o.rootCmd, "./site/src/content/docs/commands", prependTitle, linkHandler)
}

func addHiddenDummyFlag(cmd *cobra.Command, flagDummy string) {
	if cmd.PersistentFlags().Lookup(flagDummy) == nil {
		var dummyStr string
		cmd.PersistentFlags().StringVar(&dummyStr, flagDummy, "", "")
		err := cmd.PersistentFlags().MarkHidden(flagDummy)
		if err != nil {
			logger.From(cmd.Context()).Debug("Unable to add hidden dummy flag", "error", err)
		}
	}
}

// InternalCreateReadOnlyGiteaUserOptions holds the command-line options for 'internal create-read-only-gitea-user' sub-command.
type InternalCreateReadOnlyGiteaUserOptions struct{}

// NewInternalCreateReadOnlyGiteaUserCommand creates the `internal create-read-oly-gitea-user` sub-command.
func NewInternalCreateReadOnlyGiteaUserCommand() *cobra.Command {
	o := &InternalCreateReadOnlyGiteaUserOptions{}

	cmd := &cobra.Command{
		Use:   "create-read-only-gitea-user",
		Short: lang.CmdInternalCreateReadOnlyGiteaUserShort,
		Long:  lang.CmdInternalCreateReadOnlyGiteaUserLong,
		RunE:  o.Run,
	}

	return cmd
}

// Run performs the execution of 'internal create-read-only-gitea-user' sub-command.
func (o *InternalCreateReadOnlyGiteaUserOptions) Run(cmd *cobra.Command, _ []string) error {
	timeoutCtx, cancel := context.WithTimeout(cmd.Context(), cluster.DefaultTimeout)
	defer cancel()
	c, err := cluster.NewClusterWithWait(timeoutCtx)
	if err != nil {
		return err
	}
	state, err := c.LoadZarfState(cmd.Context())
	if err != nil {
		return err
	}
	tunnel, err := c.NewTunnel(cluster.ZarfNamespaceName, cluster.SvcResource, cluster.ZarfGitServerName, "", 0, cluster.ZarfGitServerPort)
	if err != nil {
		return err
	}
	_, err = tunnel.Connect(cmd.Context())
	if err != nil {
		return err
	}
	defer tunnel.Close()
	tunnelURL := tunnel.HTTPEndpoint()
	giteaClient, err := gitea.NewClient(tunnelURL, state.GitServer.PushUsername, state.GitServer.PushPassword)
	if err != nil {
		return err
	}
	err = tunnel.Wrap(func() error {
		err = giteaClient.CreateReadOnlyUser(cmd.Context(), state.GitServer.PullUsername, state.GitServer.PullPassword)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// InternalCreateArtifactRegistryTokenOptions holds the command-line options for 'internal create-artifact-registry-token' sub-command.
type InternalCreateArtifactRegistryTokenOptions struct{}

// NewInternalCreateArtifactRegistryTokenCommand creates the `internal create-artifact-registry-token` sub-command.
func NewInternalCreateArtifactRegistryTokenCommand() *cobra.Command {
	o := &InternalCreateArtifactRegistryTokenOptions{}

	cmd := &cobra.Command{
		Use:   "create-artifact-registry-token",
		Short: lang.CmdInternalArtifactRegistryGiteaTokenShort,
		Long:  lang.CmdInternalArtifactRegistryGiteaTokenLong,
		RunE:  o.Run,
	}

	return cmd
}

// Run performs the execution of 'internal create-artifact-registry-token' sub-command.
func (o *InternalCreateArtifactRegistryTokenOptions) Run(cmd *cobra.Command, _ []string) error {
	timeoutCtx, cancel := context.WithTimeout(cmd.Context(), cluster.DefaultTimeout)
	defer cancel()
	c, err := cluster.NewClusterWithWait(timeoutCtx)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	state, err := c.LoadZarfState(ctx)
	if err != nil {
		return err
	}

	// If we are setup to use an internal artifact server, create the artifact registry token
	if state.ArtifactServer.IsInternal() {
		tunnel, err := c.NewTunnel(cluster.ZarfNamespaceName, cluster.SvcResource, cluster.ZarfGitServerName, "", 0, cluster.ZarfGitServerPort)
		if err != nil {
			return err
		}
		_, err = tunnel.Connect(cmd.Context())
		if err != nil {
			return err
		}
		defer tunnel.Close()
		tunnelURL := tunnel.HTTPEndpoint()
		giteaClient, err := gitea.NewClient(tunnelURL, state.GitServer.PushUsername, state.GitServer.PushPassword)
		if err != nil {
			return err
		}
		err = tunnel.Wrap(func() error {
			tokenSha1, err := giteaClient.CreatePackageRegistryToken(ctx)
			if err != nil {
				return fmt.Errorf("unable to create an artifact registry token for Gitea: %w", err)
			}
			state.ArtifactServer.PushToken = tokenSha1
			return nil
		})
		if err != nil {
			return err
		}
		if err := c.SaveZarfState(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

// InternalUpdateGiteaPVCOptions holds the command-line options for 'internal update-gitea-pvc' sub-command.
type InternalUpdateGiteaPVCOptions struct {
	rollback bool
}

// NewInternalUpdateGiteaPVCCommand creates the `internal update-gitea-pvc` sub-command.
func NewInternalUpdateGiteaPVCCommand() *cobra.Command {
	o := &InternalUpdateGiteaPVCOptions{}

	cmd := &cobra.Command{
		Use:   "update-gitea-pvc",
		Short: lang.CmdInternalUpdateGiteaPVCShort,
		Long:  lang.CmdInternalUpdateGiteaPVCLong,
		RunE:  o.Run,
	}

	cmd.Flags().BoolVarP(&o.rollback, "rollback", "r", false, lang.CmdInternalFlagUpdateGiteaPVCRollback)

	return cmd
}

// Run performs the execution of 'internal update-gitea-pvc' sub-command.
func (o *InternalUpdateGiteaPVCOptions) Run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	pvcName := os.Getenv("ZARF_VAR_GIT_SERVER_EXISTING_PVC")

	c, err := cluster.NewCluster()
	if err != nil {
		return err
	}
	// There is a possibility that the pvc does not yet exist and Gitea helm chart should create it
	helmShouldCreate, err := c.UpdateGiteaPVC(ctx, pvcName, o.rollback)
	if err != nil {
		message.WarnErr(err, lang.CmdInternalUpdateGiteaPVCErr)
		logger.From(ctx).Warn("Unable to update the existing Gitea persistent volume claim", "error", err.Error())
	}
	fmt.Print(helmShouldCreate)
	return nil
}

// InternalIsValidHostnameOptions holds the command-line options for 'internal is-valid-hostname' sub-command.
type InternalIsValidHostnameOptions struct{}

// NewInternalIsValidHostnameCommand creates the `internal is-valid-hostname` sub-command.
func NewInternalIsValidHostnameCommand() *cobra.Command {
	o := &InternalIsValidHostnameOptions{}

	cmd := &cobra.Command{
		Use:   "is-valid-hostname",
		Short: lang.CmdInternalIsValidHostnameShort,
		RunE:  o.Run,
	}

	return cmd
}

// Run performs the execution of 'internal is-valid-hostname' sub-command.
func (o *InternalIsValidHostnameOptions) Run(_ *cobra.Command, _ []string) error {
	if valid := helpers.IsValidHostName(); !valid {
		hostname, err := os.Hostname()
		return fmt.Errorf("the hostname %s is not valid. Ensure the hostname meets RFC1123 requirements https://www.rfc-editor.org/rfc/rfc1123.html, error=%w", hostname, err)
	}
	return nil
}

// InternalCrc32Options holds the command-line options for 'intenral crc32' sub-command.
type InternalCrc32Options struct{}

// NewInternalCrc32Command creates the `internal crc32` sub-command.
func NewInternalCrc32Command() *cobra.Command {
	o := &InternalCrc32Options{}

	cmd := &cobra.Command{
		Use:     "crc32 TEXT",
		Aliases: []string{"c"},
		Short:   lang.CmdInternalCrc32Short,
		Args:    cobra.ExactArgs(1),
		Run:     o.Run,
	}

	return cmd
}

// Run performs the execution of 'internal crc32' sub-command.
func (o *InternalCrc32Options) Run(_ *cobra.Command, args []string) {
	text := args[0]
	hash := helpers.GetCRCHash(text)
	fmt.Printf("%d\n", hash)
}
