package packager

import (
	"os"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/images"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func Deploy(packageName string, confirm bool, featureRequest string) {
	tempPath := createPaths()

	if utils.InvalidPath(packageName) {
		logrus.WithField("archive", packageName).Fatal("The package archive seems to be missing or unreadable.")
	}

	// Don't continue unless the user says so
	if !confirmDeployment(packageName, tempPath, confirm) {
		os.Exit(0)
	}

	logrus.Info("Extracting the package, this may take a few moments")

	// Extract the archive
	err := archiver.Unarchive(packageName, tempPath.base)
	if err != nil {
		logrus.Fatal("Unable to extract the package contents")
	}

	// Load the config from the extracted archive zarf-config.yaml
	config.DynamicConfigLoad(tempPath.base + "/zarf-config.yaml")

	remoteImageList := config.GetRemoteImages()
	remoteRepoList := config.GetRemoteRepos()

	deployLocalAssets(tempPath, config.ZarfFeature{
		Charts:    config.GetLocalCharts(),
		Files:     config.GetLocalFiles(),
		Images:    config.GetLocalImages(),
		Manifests: config.GetLocalManifests(),
	})

	// Don't process remote for init config packages
	if !config.IsZarfInitConfig() {
		if len(remoteImageList) > 0 {
			logrus.Info("Loading images for remote install")
			// Push all images the images.tar file based on the zarf-config.yaml list
			images.PushAll(tempPath.remoteImage, remoteImageList, config.ZarfLocal)
		}

		if len(remoteRepoList) > 0 {
			logrus.Info("Loading git repos for remote install")
			// Push all the repos from the extracted archive
			git.PushAllDirectories(tempPath.remoteRepos)
		}
	} else {
		features := config.GetInitFeatures()
		for _, feature := range features {
			var confirmFeature bool
			// Only run the prompt if no features were passed in
			if featureRequest == "" {
				prompt := &survey.Confirm{
					Message: "Deploy the " + feature.Name + " feature?",
					Default: feature.Default,
					Help:    feature.Description,
				}
				_ = survey.AskOne(prompt, &confirmFeature)
			} else {
				// This is probably sufficient for now, we could change to a slice and match exact if it's needed
				confirmFeature = strings.Contains(strings.ToLower(featureRequest), feature.Name)
			}
			if confirmFeature {
				featurePath := createFeaturePaths(tempPath.features, feature)
				deployLocalAssets(featurePath, feature)
			}
		}
	}

	cleanup(tempPath)
}

func confirmDeployment(packageName string, tempPath tempPaths, confirm bool) bool {
	// Extract the config file
	_ = archiver.Extract(packageName, "zarf-config.yaml", tempPath.base)
	configPath := tempPath.base + "/zarf-config.yaml"
	confirm = confirmAction(configPath, confirm, "Deploy")
	_ = os.Remove(configPath)
	return confirm
}

func deployLocalAssets(tempPath tempPaths, assets config.ZarfFeature) {
	if assets.Name != "" {
		// Only log this for named features
		logrus.WithField("feature", assets.Name).Info("Deploying Zarf feature")
		assets.Name = "core"
	}
	if len(assets.Files) > 0 {
		logrus.Info("Loading files for local install")
		for index, file := range assets.Files {
			sourceFile := tempPath.localFiles + "/" + strconv.Itoa(index)
			err := copy.Copy(sourceFile, file.Target)
			if err != nil {
				logrus.WithField("file", file.Target).Fatal("Unable to copy the contents of the asset")
			}
		}
	}

	if len(assets.Charts) > 0 {
		logrus.Info("Loading charts for local install")
		for _, chart := range assets.Charts {
			target := "/" + chart.Name + "-" + chart.Version + ".tgz"
			utils.CreatePathAndCopy(tempPath.localCharts+target, config.K3sChartPath+target)
		}
	}

	if len(assets.Images) > 0 {
		logrus.Info("Loading images for local install")
		if config.IsZarfInitConfig() {
			utils.CreatePathAndCopy(tempPath.localImage, config.K3sImagePath+"/images-"+assets.Name+".tar")
		} else {
			_, err := utils.ExecCommand(nil, "/usr/local/bin/k3s", "ctr", "images", "import", tempPath.localImage)
			if err != nil {
				logrus.Fatal("Unable to import the images into containerd")
			}
		}
	}

	if assets.Manifests != "" {
		logrus.Info("Loading manifests for local install, this may take a minute or so to reflect in k3s")
		utils.CreatePathAndCopy(tempPath.localManifests, config.K3sManifestPath)
	}
}
