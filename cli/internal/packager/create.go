package packager

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/helm"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/images"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func Create(packageName string, confirm bool) {
	tempPath := createPaths()
	dataInjections := config.GetDataInjections()
	remoteImageList := config.GetRemoteImages()
	remoteRepoList := config.GetRemoteRepos()
	features := config.GetInitFeatures()
	configFile := tempPath.base + "/zarf-config.yaml"

	// Save the transformed config
	config.WriteConfig(configFile)

	confirm = confirmAction(configFile, confirm, "Create")

	if !confirm {
		os.Exit(0)
	}

	addLocalAssets(tempPath, config.ZarfFeature{
		Charts:    config.GetLocalCharts(),
		Files:     config.GetLocalFiles(),
		Images:    config.GetLocalImages(),
		Manifests: config.GetLocalManifests(),
	})

	for _, feature := range features {
		logrus.WithField("feature", feature.Name).Info("Loading feature assets")
		featurePath := createFeaturePaths(tempPath.features, feature)
		addLocalAssets(featurePath, feature)
	}

	// Init config ignore remote entries
	if !config.IsZarfInitConfig() {
		if len(dataInjections) > 0 {
			logrus.Info("Loading data injections")
			for _, data := range dataInjections {
				destinationFile := tempPath.dataInjections + "/" + filepath.Base(data.Target.Path)
				utils.CreatePathAndCopy(data.Source, destinationFile)
			}
		}

		if len(remoteImageList) > 0 {
			logrus.Info("Loading images for remote install")
			images.PullAll(remoteImageList, tempPath.remoteImage)
		}

		if len(remoteRepoList) > 0 {
			logrus.Info("loading git repos for remote install")
			// Load all specified git repos
			for _, url := range remoteRepoList {
				matches := strings.Split(url, "@")
				if len(matches) < 2 {
					logrus.WithField("remote", url).Fatal("Unable to parse git url. Ensure you use the format url.git@tag")
				}
				git.Pull(matches[0], tempPath.remoteRepos)
			}
		}
	}
	_ = os.RemoveAll(packageName)
	err := archiver.Archive([]string{tempPath.base + "/"}, packageName)
	if err != nil {
		logrus.Fatal("Unable to create the package archive")
	}

	logrus.WithField("name", packageName).Info("Package creation complete")

	cleanup(tempPath)
}

func addLocalAssets(tempPath tempPaths, assets config.ZarfFeature) {
	if len(assets.Charts) > 0 {
		logrus.Info("Loading static helm charts")
		utils.CreateDirectory(tempPath.localCharts, 0700)
		for _, chart := range assets.Charts {
			isGitURL, _ := regexp.MatchString("\\.git$", chart.Url)
			if isGitURL {
				helm.DownloadChartFromGit(chart, tempPath.localCharts)
			} else {
				helm.DownloadPublishedChart(chart, tempPath.localCharts)
			}
		}
	}

	if len(assets.Files) > 0 {
		logrus.Info("Downloading files for local install")
		_ = utils.CreateDirectory(tempPath.localFiles, 0700)
		for index, file := range assets.Files {
			destinationFile := tempPath.localFiles + "/" + strconv.Itoa(index)
			utils.DownloadToFile(file.Url, destinationFile)
			if file.Executable {
				_ = os.Chmod(destinationFile, 0700)
			} else {
				_ = os.Chmod(destinationFile, 0600)
			}
		}
	}

	if len(assets.Images) > 0 {
		logrus.Info("Loading images for local install")
		images.PullAll(assets.Images, tempPath.localImage)
	}

	if assets.Manifests != "" {
		logrus.WithField("path", assets.Manifests).Info("Loading manifests for local install")
		utils.CreatePathAndCopy(assets.Manifests, tempPath.localManifests)
	}
}
