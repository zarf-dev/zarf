package packager

import (
	"os"

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

	// Extract the archive
	archiver.Unarchive(packageName, tempPath.base)

	// Load the config from the extracted archive config.yaml
	config.DynamicConfigLoad(tempPath.base)

	localBinaries := config.GetLocalBinaries()
	localImageList := config.GetLocalImages()
	localManifestPath := config.GetLocalManifests()
	remoteImageList := config.GetRemoteImages()
	remoteRepoList := config.GetRemoteRepos()

	if localBinaries != nil {
		logrus.Info("Loading binaries for local install")
		for _, binary := range localBinaries {
			sourceFile := tempPath.localBin + "/" + binary.Name
			destinationFile := "/usr/local/bin/" + binary.Name
			err := copy.Copy(sourceFile, destinationFile)
			if err != nil {
				logrus.WithField("binary", binary.Name).Fatal("Unable to copy the contens of the asset")
			}
			_ = os.Chmod(destinationFile, 0700)
		}
	}

	if localImageList != nil {
		logrus.Info("Loading images for local install")
		git.PushAllDirectories(tempPath.remoteRepos, "https://"+targetUrl)
		utils.CreatePathAndCopy(tempPath.localImage, config.K3sImagePath+"/images.tar")
	}

	if localManifestPath != "" {
		logrus.Info("Loading manifests for local install")
		utils.CreatePathAndCopy(tempPath.localManifests, config.K3sManifestPath)
	}

	if remoteImageList != nil {
		logrus.Info("Loading images for remote install")
		// Push all images the images.tar file based on the config.yaml list
		images.PushAll(tempPath.localImage, remoteImageList, targetUrl)
	}

	if remoteRepoList != nil {
		logrus.Info("Loading git repos for remote install")
		// Push all the repos from the extracted archive
		git.PushAllDirectories(tempPath.remoteRepos, "https://"+targetUrl)
	}

	cleanup(tempPath)
}
