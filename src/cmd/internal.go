package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/alecthomas/jsonschema"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var internalCmd = &cobra.Command{
	Use:     "internal",
	Aliases: []string{"dev", "int"},
	Hidden:  true,
	Short:   "Internal tools used by zarf",
}

var generateCLIDocs = &cobra.Command{
	Use:   "generate-cli-docs",
	Short: "Creates auto-generated markdown of all the commands for the CLI",
	Run: func(cmd *cobra.Command, args []string) {
		utils.CreateDirectory("clidocs", 0700)

		//Generate markdown of the Zarf command (and all of its child commands)
		doc.GenMarkdownTree(rootCmd, "./docs/4-user-guide/1-the-zarf-cli/100-cli-commands")
	},
}

var configSchemaCmd = &cobra.Command{
	Use:     "config-schema",
	Aliases: []string{"c"},
	Short:   "Generates a JSON schema for the zarf.yaml configuration",
	Run: func(cmd *cobra.Command, args []string) {
		schema := jsonschema.Reflect(&types.ZarfPackage{})
		output, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			message.Fatal(err, "Unable to generate the zarf config schema")
		}
		fmt.Print(string(output) + "\n")
	},
}

var createReadOnlyGiteaUser = &cobra.Command{
	Use:   "create-read-only-gitea-user",
	Short: "Creates a read-only user in Gitea",
	Long: "Creates a read-only user in Gitea by using the Gitea API. " +
		"This is called internally by the supported Gitea package component.",
	Run: func(cmd *cobra.Command, args []string) {
		// Load the state so we can get the credentials for the admin git user
		state := k8s.LoadZarfState()
		config.InitState(state)

		// Create the non-admin user
		err := git.CreateReadOnlyUser()
		if err != nil {
			message.Error(err, "Unable to create a read-only user in the Gitea service.")
		}
	},
}

func init() {
	rootCmd.AddCommand(internalCmd)

	internalCmd.AddCommand(generateCLIDocs)
	internalCmd.AddCommand(configSchemaCmd)
	internalCmd.AddCommand(createReadOnlyGiteaUser)
}
