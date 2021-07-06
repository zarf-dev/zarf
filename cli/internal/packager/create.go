package packager

import (
	"os"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/images"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func Create(packageName string) {
	tempPath := createPaths()
	localBinaries := config.GetLocalBinaries()
	localImageList := config.GetLocalImages()
	localManifestPath := config.GetLocalManifests()
	remoteImageList := config.GetRemoteImages()
	remoteRepoList := config.GetRemoteRepos()

	// Bundle all assets into compressed tarball
	sourceFiles := []string{"config.yaml"}

	// @TODO implement the helm pull functionality directly into the CLI
	if config.IsZarfInitConfig() && !utils.InvalidPath("charts") {
		logrus.Info("Loading static helm charts")
		sourceFiles = append(sourceFiles, tempPath.localCharts)
		utils.CreatePathAndCopy("charts", tempPath.localCharts)
	}

	if len(localBinaries) > 0 {
		logrus.Info("Loading binaries for local install")
		sourceFiles = append(sourceFiles, tempPath.localBin)
		_ = utils.CreateDirectory(tempPath.localBin, 0700)
		for _, binary := range localBinaries {
			destinationFile := tempPath.localBin + "/" + binary.Name
			utils.DownloadFile(binary.Url, destinationFile)
			_ = os.Chmod(destinationFile, 0700)
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
	archiver.Archive(sourceFiles, packageName)

	cleanup(tempPath)
}
