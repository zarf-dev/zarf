// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alecthomas/jsonschema"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/agent"
	"github.com/defenseunicorns/zarf/src/internal/api"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var internalCmd = &cobra.Command{
	Use:     "internal",
	Aliases: []string{"dev"},
	Hidden:  true,
	Short:   lang.CmdInternalShort,
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: lang.CmdInternalAgentShort,
	Long:  lang.CmdInternalAgentLong,
	Run: func(cmd *cobra.Command, args []string) {
		agent.StartWebhook()
	},
}

var generateCLIDocs = &cobra.Command{
	Use:   "generate-cli-docs",
	Short: lang.CmdInternalGenerateCliDocsShort,
	Run: func(cmd *cobra.Command, args []string) {
		// Don't include the datestamp in the output
		rootCmd.DisableAutoGenTag = true
		//Generate markdown of the Zarf command (and all of its child commands)
		if err := os.RemoveAll("./docs/4-user-guide/1-the-zarf-cli/100-cli-commands"); err != nil {
			message.Fatalf("Unable to generate the CLI documentation: %s", err.Error())
		}
		if err := os.Mkdir("./docs/4-user-guide/1-the-zarf-cli/100-cli-commands", 0775); err != nil {
			message.Fatalf("Unable to generate the CLI documentation: %s", err.Error())
		}
		if err := doc.GenMarkdownTree(rootCmd, "./docs/4-user-guide/1-the-zarf-cli/100-cli-commands"); err != nil {
			message.Fatalf("Unable to generate the CLI documentation: %s", err.Error())
		} else {
			message.Successf(lang.CmdInternalGenerateCliDocsSuccess)
		}
	},
}

var configSchemaCmd = &cobra.Command{
	Use:     "config-schema",
	Aliases: []string{"c"},
	Short:   lang.CmdInternalConfigSchemaShort,
	Run: func(cmd *cobra.Command, args []string) {
		schema := jsonschema.Reflect(&types.ZarfPackage{})
		output, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			message.Fatal(err, lang.CmdInternalConfigSchemaErr)
		}
		fmt.Print(string(output) + "\n")
	},
}

var apiSchemaCmd = &cobra.Command{
	Use:   "api-schema",
	Short: lang.CmdInternalAPISchemaShort,
	Run: func(cmd *cobra.Command, args []string) {
		schema := jsonschema.Reflect(&types.RestAPI{})
		output, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			message.Fatal(err, lang.CmdInternalAPISchemaGenerateErr)
		}
		fmt.Print(string(output) + "\n")
	},
}

var createReadOnlyGiteaUser = &cobra.Command{
	Use:   "create-read-only-gitea-user",
	Short: lang.CmdInternalCreateReadOnlyGiteaUserShort,
	Long:  lang.CmdInternalCreateReadOnlyGiteaUserLong,
	Run: func(cmd *cobra.Command, args []string) {
		// Load the state so we can get the credentials for the admin git user
		state, err := cluster.NewClusterOrDie().LoadZarfState()
		if err != nil {
			message.Error(err, lang.CmdInternalCreateReadOnlyGiteaUserErr)
		}

		// Create the non-admin user
		if err = git.New(state.GitServer).CreateReadOnlyUser(); err != nil {
			message.Error(err, lang.CmdInternalCreateReadOnlyGiteaUserErr)
		}
	},
}

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: lang.CmdInternalUIShort,
	Run: func(cmd *cobra.Command, args []string) {
		api.LaunchAPIServer()
	},
}

var isValidHostname = &cobra.Command{
	Use:   "is-valid-hostname",
	Short: lang.CmdInternalIsValidHostnameShort,
	Run: func(cmd *cobra.Command, args []string) {
		if valid := utils.IsValidHostName(); !valid {
			hostname, _ := os.Hostname()
			message.Fatalf(nil, "The hostname '%s' is not valid. Ensure the hostname meets RFC1123 requirements https://www.rfc-editor.org/rfc/rfc1123.html.", hostname)
		}
	},
}

func init() {
	rootCmd.AddCommand(internalCmd)

	internalCmd.AddCommand(agentCmd)
	internalCmd.AddCommand(generateCLIDocs)
	internalCmd.AddCommand(configSchemaCmd)
	internalCmd.AddCommand(apiSchemaCmd)
	internalCmd.AddCommand(createReadOnlyGiteaUser)
	internalCmd.AddCommand(uiCmd)
	internalCmd.AddCommand(isValidHostname)
}
