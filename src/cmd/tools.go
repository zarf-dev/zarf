package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/alecthomas/jsonschema"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	k9s "github.com/derailed/k9s/cmd"
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var toolsCmd = &cobra.Command{
	Use:     "tools",
	Aliases: []string{"t"},
	Short:   "Collection of additional tools to make airgap easier",
}

// destroyCmd represents the init command
var archiverCmd = &cobra.Command{
	Use:     "archiver",
	Aliases: []string{"a"},
	Short:   "Compress/Decompress tools",
}

var archiverCompressCmd = &cobra.Command{
	Use:     "compress [SOURCES] [ARCHIVE]",
	Aliases: []string{"c"},
	Short:   "Compress a collection of sources based off of the destination file extension",
	Args:    cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceFiles, destinationArchive := args[:len(args)-1], args[len(args)-1]
		err := archiver.Archive(sourceFiles, destinationArchive)
		if err != nil {
			message.Fatal(err, "Unable to perform compression")
		}
	},
}

var archiverDecompressCmd = &cobra.Command{
	Use:     "decompress [ARCHIVE] [DESTINATION]",
	Aliases: []string{"d"},
	Short:   "Decompress an archive to a specified location.",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceArchive, destinationPath := args[0], args[1]
		err := archiver.Unarchive(sourceArchive, destinationPath)
		if err != nil {
			message.Fatal(err, "Unable to perform decompression")
		}
	},
}

var registryCmd = &cobra.Command{
	Use:     "registry",
	Aliases: []string{"r"},
	Short:   "Collection of registry commands provided by Crane",
}

var readCredsCmd = &cobra.Command{
	Use:   "get-admin-password",
	Short: "Returns the Zarf admin password for gitea read from the zarf-state secret in the zarf namespace",
	Run: func(cmd *cobra.Command, args []string) {
		state := k8s.LoadZarfState()
		if state.Distro == "" {
			// If no distro the zarf secret did not load properly
			message.Fatalf(nil, "Unable to load the zarf/zarf-state secret, did you remember to run zarf init first?")
		}

		// Continue loading state data if it is valid
		config.InitState(state)

		fmt.Println(config.GetSecret(config.StateGitPush))
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

var k9sCmd = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"m", "k9s"},
	Short:   "Launch K9s tool for managing K8s clusters",
	Run: func(cmd *cobra.Command, args []string) {
		// Hack to make k9s think it's all alone
		os.Args = []string{os.Args[0], "-n", "zarf"}
		k9s.Execute()
	},
}

var createReadOnlyGiteaUser = &cobra.Command{
	Use:    "create-read-only-gitea-user",
	Hidden: true,
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

var generateCLIDocs = &cobra.Command{
	Use:    "generate-cli-docs",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		utils.CreateDirectory("clidocs", 0700)

		//Generate markdown of the Zarf command (and all of its child commands)
		doc.GenMarkdownTree(rootCmd, "./clidocs")
	},
}

func init() {
	rootCmd.AddCommand(toolsCmd)
	rootCmd.AddCommand(generateCLIDocs)

	toolsCmd.AddCommand(archiverCmd)
	toolsCmd.AddCommand(readCredsCmd)
	toolsCmd.AddCommand(configSchemaCmd)
	toolsCmd.AddCommand(k9sCmd)
	toolsCmd.AddCommand(registryCmd)
	toolsCmd.AddCommand(createReadOnlyGiteaUser)

	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)

	cranePlatformOptions := []crane.Option{config.GetCraneOptions()}
	registryCmd.AddCommand(craneCmd.NewCmdAuthLogin())
	registryCmd.AddCommand(craneCmd.NewCmdPull(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdPush(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCopy(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCatalog(&cranePlatformOptions))
}
