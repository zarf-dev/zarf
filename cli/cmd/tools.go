package cmd

import (
	"fmt"

	craneCmd "github.com/google/go-containerregistry/cmd/crane/cmd"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
)

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Collection of additional tools to make airgap easier",
}

// destroyCmd represents the init command
var archiverCmd = &cobra.Command{
	Use:   "archiver",
	Short: "Compress/Decompress tools",
}

var archiverCompressCmd = &cobra.Command{
	Use:   "compress SOURCES ARCHIVE",
	Short: "Compress a collection of sources based off of the destination file extension",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceFiles, destinationArchive := args[:len(args)-1], args[len(args)-1]
		err := archiver.Archive(sourceFiles, destinationArchive)
		if err != nil {
			logrus.Fatal(err)
		}
	},
}

var archiverDecompressCmd = &cobra.Command{
	Use:   "decompress ARCHIVE DESTINATION",
	Short: "Decompress an archive to a specified location.",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		sourceArchive, destinationPath := args[0], args[1]
		err := archiver.Unarchive(sourceArchive, destinationPath)
		if err != nil {
			logrus.Fatal(err)
		}
	},
}

var registryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Collection of registry commands provided by Crane",
}

var readCredsCmd = &cobra.Command{
	Use:   "get-admin-password",
	Short: "Returns the Zarf admin password read from ~/.git-credentials",
	Run: func(cmd *cobra.Command, args []string) {
		authInfo := git.FindAuthForHost(config.ZarfLocalIP)
		fmt.Println(authInfo.Auth.Password)
	},
}

func init() {
	rootCmd.AddCommand(toolsCmd)

	toolsCmd.AddCommand(archiverCmd)
	toolsCmd.AddCommand(readCredsCmd)
	archiverCmd.AddCommand(archiverCompressCmd)
	archiverCmd.AddCommand(archiverDecompressCmd)

	toolsCmd.AddCommand(registryCmd)
	cranePlatformOptions := []crane.Option{
		crane.WithPlatform(&v1.Platform{OS: "linux", Architecture: "amd64"}),
	}
	registryCmd.AddCommand(craneCmd.NewCmdAuthLogin())
	registryCmd.AddCommand(craneCmd.NewCmdPull(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdPush(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCopy(&cranePlatformOptions))
	registryCmd.AddCommand(craneCmd.NewCmdCatalog(&cranePlatformOptions))
}
