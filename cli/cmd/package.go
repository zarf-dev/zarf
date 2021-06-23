package cmd

import (
	"os"
	"strings"

	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/images"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var packageRepoList []string
var packageImageList []string

const exportName = "zarf-update.tar.zst"

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Pack and unpack updates for the Zarf utility cluster.",
}

// packageCreateCmd represents the build command
var packageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an update package to push to the utility server (runs online)",
	Run: func(cmd *cobra.Command, args []string) {
		basePath := utils.MakeTempDir()

		logrus.Info("Loading git repos")
		for _, url := range packageRepoList {
			matches := strings.Split(url, "@")
			if len(matches) < 2 {
				logrus.WithField("remote", url).Fatal("Unable to parse git url. Ensure you use the format url.git@tag")
			}
			git.Pull(matches[0], basePath+"/repos/", matches[1])
		}

		images.PullAll(packageImageList, basePath+"/images.tar")

		sourceFiles, destinationArchive := []string{
			"config.yaml",
			basePath + "/repos",
			basePath + "/images.tar",
		}, exportName

		_ = os.RemoveAll(exportName)
		utils.Compress(sourceFiles, destinationArchive)

		logrus.Info("Cleaning up temp files")
		_ = os.RemoveAll(basePath)
	},
}

// packageDeployCmd represents the build command
var packageDeployCmd = &cobra.Command{
	Use:   "deploy ARCHIVE",
	Short: "deploys a zarf-update.tar.zst file (runs offline)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		basePath := utils.MakeTempDir()
		utils.Decompress(args[0], basePath)

		images.PushAll(basePath+"/images.tar", packageImageList, "localhost:8443")

		logrus.Info("Cleaning up temp files")
		_ = os.RemoveAll(basePath)
	},
}

func init() {
	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCreateCmd.Flags().StringSliceVarP(&packageRepoList, "repo", "r", config.GetRepos(), "")
	packageCreateCmd.Flags().StringSliceVarP(&packageImageList, "images", "i", config.GetImages(), "")
	packageDeployCmd.Flags().StringSliceVarP(&packageRepoList, "repo", "r", config.GetRepos(), "")
	packageDeployCmd.Flags().StringSliceVarP(&packageImageList, "images", "i", config.GetImages(), "")
}
