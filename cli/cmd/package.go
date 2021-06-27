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

type TempPaths struct {
	base  string
	image string
	repos string
}

const updatePackageName = "zarf-update.tar.zst"

var packageCmd = &cobra.Command{
	Use:   "package",
	Short: "Pack and unpack updates for the Zarf utility cluster.",
}

// packageCreateCmd represents the build command
var packageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an update package to push to the utility server (runs online)",
	Run: func(cmd *cobra.Command, args []string) {
		tempPath := createPaths()

		logrus.Info("Loading git repos")
		for _, url := range config.GetRepos() {
			matches := strings.Split(url, "@")
			if len(matches) < 2 {
				logrus.WithField("remote", url).Fatal("Unable to parse git url. Ensure you use the format url.git@tag")
			}
			git.Pull(matches[0], tempPath.repos, matches[1])
		}

		images.PullAll(config.GetImages(), tempPath.image)

		sourceFiles, destinationArchive := []string{
			"config.yaml",
			tempPath.repos,
			tempPath.image,
		}, updatePackageName

		_ = os.RemoveAll(updatePackageName)
		utils.Compress(sourceFiles, destinationArchive)

		cleanup(tempPath)
	},
}

// packageDeployCmd represents the build command
var packageDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploys a " + updatePackageName + " file (runs offline)",
	Run: func(cmd *cobra.Command, args []string) {
		tempPath := createPaths()
		targetUrl := "zarf.localhost"
		imageList := config.GetImages()

		utils.Decompress(updatePackageName, tempPath.base)

		git.PushAllDirectories(tempPath.repos, "https://"+targetUrl)

		images.PushAll(tempPath.image, imageList, targetUrl)

		cleanup(tempPath)
	},
}

func createPaths() TempPaths {
	basePath := utils.MakeTempDir()
	return TempPaths{
		base:  basePath,
		image: basePath + "/images.tar",
		repos: basePath + "/repos",
	}
}

func cleanup(tempPath TempPaths) {
	logrus.Info("Cleaning up temp files")
	_ = os.RemoveAll(tempPath.base)
}

func init() {
	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
}
