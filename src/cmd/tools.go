package cmd

import (
	"fmt"
	"os"

	"github.com/anchore/syft/cmd/syft/cli"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	k9s "github.com/derailed/k9s/cmd"
	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
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
	Aliases: []string{"r", "crane"},
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

func init() {
	rootCmd.AddCommand(toolsCmd)

	toolsCmd.AddCommand(archiverCmd)
	toolsCmd.AddCommand(readCredsCmd)
	toolsCmd.AddCommand(k9sCmd)
	toolsCmd.AddCommand(registryCmd)

	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)

	cranePlatformOptions := config.GetCraneOptions()

	craneLogin := craneCmd.NewCmdAuthLogin()
	craneLogin.Example = ""

	registryCmd.AddCommand(craneLogin)
	registryCmd.AddCommand(craneCmd.NewCmdPull(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdPush(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCopy(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCatalog(&cranePlatformOptions))

	syftCmd, err := cli.New()
	if err != nil {
		message.Fatal(err, "Unable to create sbom (syft) CLI")
	}
	syftCmd.Use = "sbom"
	syftCmd.Short = "SBOM tools provided by Anchore Syft"
	syftCmd.Aliases = []string{"s", "syft"}
	syftCmd.Example = `  zarf tools sbom packages alpine:latest                                a summary of discovered packages
  zarf tools sbom packages alpine:latest -o json                        show all possible cataloging details
  zarf tools sbom packages alpine:latest -o cyclonedx                   show a CycloneDX formatted SBOM
  zarf tools sbom packages alpine:latest -o cyclonedx-json              show a CycloneDX JSON formatted SBOM
  zarf tools sbom packages alpine:latest -o spdx                        show a SPDX 2.2 Tag-Value formatted SBOM
  zarf tools sbom packages alpine:latest -o spdx-json                   show a SPDX 2.2 JSON formatted SBOM
  zarf tools sbom packages alpine:latest -vv                            show verbose debug information
  zarf tools sbom packages alpine:latest -o template -t my_format.tmpl  show a SBOM formatted according to given template file

  Supports the following image sources:
    zarf tools sbom packages yourrepo/yourimage:tag     defaults to using images from a Docker daemon. If Docker is not present, the image is pulled directly from the registry.
    zarf tools sbom packages path/to/a/file/or/dir      a Docker tar, OCI tar, OCI directory, or generic filesystem directory

  You can also explicitly specify the scheme to use:
    zarf tools sbom packages docker:yourrepo/yourimage:tag          explicitly use the Docker daemon
    zarf tools sbom packages podman:yourrepo/yourimage:tag          explicitly use the Podman daemon
    zarf tools sbom packages registry:yourrepo/yourimage:tag        pull image directly from a registry (no container runtime required)
    zarf tools sbom packages docker-archive:path/to/yourimage.tar   use a tarball from disk for archives created from "docker save"
    zarf tools sbom packages oci-archive:path/to/yourimage.tar      use a tarball from disk for OCI archives (from Skopeo or otherwise)
    zarf tools sbom packages oci-dir:path/to/yourimage              read directly from a path on disk for OCI layout directories (from Skopeo or otherwise)
    zarf tools sbom packages dir:path/to/yourproject                read directly from a path on disk (any directory)
    zarf tools sbom packages file:path/to/yourproject/file          read directly from a path on disk (any single file)`

	for _, subCmd := range syftCmd.Commands() {
		subCmd.Example = ""
	}

	toolsCmd.AddCommand(syftCmd)
}
