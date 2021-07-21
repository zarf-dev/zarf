package packager

import (
	"strconv"

	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/images"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func Deploy(packageName string) {
	tempPath := createPaths()
	targetUrl := "zarf.localhost"

	if utils.InvalidPath(packageName) {
		logrus.WithField("archive", packageName).Fatal("The package archive seems to be missing or unreadable.")
	}

	// Extract the archive
	archiver.Unarchive(packageName, tempPath.base)

	// Load the config from the extracted archive config.yaml
	config.DynamicConfigLoad(tempPath.base + "/config.yaml")

	localFiles := config.GetLocalFiles()
	localImageList := config.GetLocalImages()
	localManifestPath := config.GetLocalManifests()
	remoteImageList := config.GetRemoteImages()
	remoteRepoList := config.GetRemoteRepos()

	if len(localFiles) > 0 {
		logrus.Info("Loading files for local install")
		for index, file := range localFiles {
			sourceFile := tempPath.localFiles + "/" + strconv.Itoa(index)
			err := copy.Copy(sourceFile, file.Target)
			if err != nil {
				logrus.WithField("file", file.Target).Fatal("Unable to copy the contens of the asset")
			}
		}
	}

	// @TODO implement the helm pull functionality directly into the CLI
	if config.IsZarfInitConfig() && !utils.InvalidPath(tempPath.localCharts) {
		logrus.Info("Loading helm charts for local install")
		utils.CreatePathAndCopy(tempPath.localCharts, config.K3sChartPath)
	}

	if len(localImageList) > 0 {
		logrus.Info("Loading images for local install")
		if config.IsZarfInitConfig() {
			utils.CreatePathAndCopy(tempPath.localImage, config.K3sImagePath+"/images.tar")
		} else {
			utils.ExecCommand(nil, "/usr/local/bin/k3s", "ctr", "images", "import", tempPath.localImage)
		}
	}

	if localManifestPath != "" {
		logrus.Info("Loading manifests for local install, this may take a minute or so to reflect in k3s")
		utils.CreatePathAndCopy(tempPath.localManifests, config.K3sManifestPath)
	}

	// Don't process remote for init config packages
	if !config.IsZarfInitConfig() {
		if len(remoteImageList) > 0 {
			logrus.Info("Loading images for remote install")
			// Push all images the images.tar file based on the config.yaml list
			images.PushAll(tempPath.remoteImage, remoteImageList, targetUrl)
		}

		if len(remoteRepoList) > 0 {
			logrus.Info("Loading git repos for remote install")
			// Push all the repos from the extracted archive
			git.PushAllDirectories(tempPath.remoteRepos, "https://"+targetUrl)
		}
	}

	// cleanup(tempPath)
}
