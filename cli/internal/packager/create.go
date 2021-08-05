package packager

import (
	"os"
	"strconv"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/images"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func Create(packageName string, confirm bool) {
	tempPath := createPaths()
	localFiles := config.GetLocalFiles()
	localImageList := config.GetLocalImages()
	localManifestPath := config.GetLocalManifests()
	remoteImageList := config.GetRemoteImages()
	remoteRepoList := config.GetRemoteRepos()
	configFile := tempPath.base + "/config.yaml"

	// Save the transformed config
	config.WriteConfig(configFile)

	confirm = confirmAction(configFile, confirm, "Create")

	if !confirm {
		os.Exit(0)
	}

	// Bundle all assets into compressed tarball
	sourceFiles := []string{configFile}

	// @TODO implement the helm pull functionality directly into the CLI
	if !utils.InvalidPath("charts") {
		logrus.Info("Loading static helm charts")
		sourceFiles = append(sourceFiles, tempPath.localCharts)
		utils.CreatePathAndCopy("charts", tempPath.localCharts)
	}

	if len(localFiles) > 0 {
		logrus.Info("Downloading files for local install")
		sourceFiles = append(sourceFiles, tempPath.localFiles)
		_ = utils.CreateDirectory(tempPath.localFiles, 0700)
		for index, file := range localFiles {
			destinationFile := tempPath.localFiles + "/" + strconv.Itoa(index)
			utils.DownloadFile(file.Url, destinationFile)
			if file.Executable {
				_ = os.Chmod(destinationFile, 0700)
			} else {
				_ = os.Chmod(destinationFile, 0600)
			}
		}
	}

	if len(localImageList) > 0 {
		logrus.Info("Loading images for local install")
		sourceFiles = append(sourceFiles, tempPath.localImage)
		images.PullAll(localImageList, tempPath.localImage)
	}

	if localManifestPath != "" {
		logrus.WithField("path", localManifestPath).Info("Loading manifests for local install")
		sourceFiles = append(sourceFiles, tempPath.localManifests)
		utils.CreatePathAndCopy(localManifestPath, tempPath.localManifests)
	}

	// Init config ignore remote entries
	if !config.IsZarfInitConfig() {
		if len(remoteImageList) > 0 {
			logrus.Info("Loading images for remote install")
			sourceFiles = append(sourceFiles, tempPath.remoteImage)
			images.PullAll(remoteImageList, tempPath.remoteImage)
		}

		if len(remoteRepoList) > 0 {
			logrus.Info("loading git repos for remote install")
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
	}

	_ = os.RemoveAll(packageName)
	err := archiver.Archive(sourceFiles, packageName)
	if err != nil {
		logrus.Fatal("Unable to create the package archive")
	}

	cleanup(tempPath)
}
