package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"

	"github.com/alecthomas/jsonschema"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	k9s "github.com/derailed/k9s/cmd"
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
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
	Short:   "Compress/Decompress tools for Zarf packages",
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
	Short:   "Decompress an archive (package) to a specified location.",
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
	Short:  "Creates a read-only user in Gitea",
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

var generateCLIDocs = &cobra.Command{
	Use:    "generate-cli-docs",
	Short:  "Creates auto-generated markdown of all the commands for the CLI",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		utils.CreateDirectory("clidocs", 0700)

		//Generate markdown of the Zarf command (and all of its child commands)
		doc.GenMarkdownTree(rootCmd, "./clidocs")
	},
}

// TODO: This currently doesn't show the connect options that come out of the `init` package since those aren't handled at service annotations..
var getConnectOptions = &cobra.Command{
	Use:   "get-connect-options [FILTER]",
	Short: "Get all the connect options from the packages that have been deployed",
	Run: func(cmd *cobra.Command, args []string) {

		// Optional arg to filter the connect-name options
		connectNameFilter := ""
		if len(args) > 0 {
			connectNameFilter = args[0]
		}

		// Get ALL the services in the cluster that have the connect-name label
		allNamespaceFilter := ""
		anyValueFilter := ""
		serviceList, err := k8s.GetServicesByLabel(allNamespaceFilter, k8s.ZarfConnectLabelKey, anyValueFilter)
		if err != nil {
			message.Errorf(err, "Unable to get services that have the zarf-connect label key")
		}

		// Build up a pterm table of the resulting service connection commands
		list := pterm.TableData{{"     Connect Command", "Description"}}

		// Loop over each connecStrings and convert to pterm.TableData
		for _, service := range serviceList.Items {

			// Skip this connect option if it doesn't fit the optional name filter
			connectName := service.Labels[k8s.ZarfConnectLabelKey]
			if connectNameFilter != "" && !strings.Contains(connectName, connectNameFilter) {
				continue
			}

			// Add this connect command to the list
			connectCommand := fmt.Sprintf("     zarf connect %s", connectName)
			connectDescription := service.Annotations[k8s.ZarfConnectDescriptionKey]
			list = append(list, []string{connectCommand, connectDescription})
		}

		// Create the table output with the data if there are any matches
		if len(list) == 1 {
			message.Warn("Unable to find any connect command options in the cluster")
		} else {
			_ = pterm.DefaultTable.WithHasHeader().WithData(list).Render()
		}
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
	toolsCmd.AddCommand(getConnectOptions)

	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)

	cranePlatformOptions := config.GetCraneOptions()
	registryCmd.AddCommand(craneCmd.NewCmdAuthLogin())
	registryCmd.AddCommand(craneCmd.NewCmdPull(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdPush(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCopy(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCatalog(&cranePlatformOptions))
}
