package cmd

import (
	"os"
	"strings"

	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/images"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k3s"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type TempPaths struct {
	base           string
	localImage     string
	localManifests string
	remoteImage    string
	remoteRepos    string
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
		localImageList := config.GetLocalImages()
		localManifestPath := config.GetLocalManifests()
		remoteImageList := config.GetRemoteImages()
		remoteRepoList := config.GetRemoteRepos()

		// Bundle all assets into compressed tarball
		sourceFiles := []string{"config.yaml"}

		if remoteRepoList != nil {
			sourceFiles = append(sourceFiles, tempPath.remoteRepos)
			// Load all specified git repos
			for _, url := range remoteRepoList {
				matches := strings.Split(url, "@")
				if len(matches) < 2 {
					logrus.WithField("remote", url).Fatal("Unable to parse git url. Ensure you use the format url.git@tag")
				}
				git.Pull(matches[0], tempPath.remoteRepos, matches[1])
			}
		}

		if localImageList != nil {
			sourceFiles = append(sourceFiles, tempPath.localImage)
			images.PullAll(localImageList, tempPath.localImage)
		}

		if remoteImageList != nil {
			sourceFiles = append(sourceFiles, tempPath.remoteImage)
			images.PullAll(remoteImageList, tempPath.remoteImage)
		}

		if localManifestPath != "" {
			sourceFiles = append(sourceFiles, tempPath.localManifests)
			utils.CreatePathAndCopy(localManifestPath, tempPath.localManifests)
		}

		logrus.Warn(sourceFiles)

		_ = os.RemoveAll(updatePackageName)
		utils.Compress(sourceFiles, updatePackageName)

		// cleanup(tempPath)'
	},
}

// packageDeployCmd represents the build command
var packageDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploys a " + updatePackageName + " file (runs offline)",
	Run: func(cmd *cobra.Command, args []string) {
		tempPath := createPaths()
		targetUrl := "zarf.localhost"

		// Extract the archive
		utils.Decompress(updatePackageName, tempPath.base)

		// Load the config from the extracted archive config.yaml
		config.DynamicConfigLoad(tempPath.base)

		localImageList := config.GetLocalImages()
		localManifestPath := config.GetLocalManifests()
		remoteImageList := config.GetRemoteImages()
		remoteRepoList := config.GetRemoteRepos()

		if localImageList != nil {
			git.PushAllDirectories(tempPath.remoteRepos, "https://"+targetUrl)
			utils.CreatePathAndCopy(tempPath.localImage, k3s.K3sImagePath+"/images.tar")
		}

		if remoteRepoList != nil {
			// Push all the repos from the extracted archive
			git.PushAllDirectories(tempPath.remoteRepos, "https://"+targetUrl)
		}

		if remoteImageList != nil {
			// Push all images the images.tar file based on the config.yaml list
			images.PushAll(tempPath.localImage, remoteImageList, targetUrl)
		}

		if localManifestPath != "" {
			utils.CreatePathAndCopy(tempPath.localManifests, k3s.K3sManifestPath)
		}

		cleanup(tempPath)
	},
}

func createPaths() TempPaths {
	basePath := utils.MakeTempDir()
	return TempPaths{
		base:           basePath,
		localImage:     basePath + "/images-local.tar",
		localManifests: basePath + "/manifests",
		remoteImage:    basePath + "/images-remote.tar",
		remoteRepos:    basePath + "/repos",
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
