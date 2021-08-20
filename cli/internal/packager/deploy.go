package packager

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/config"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/git"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/images"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/k8s"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

func Deploy(packageName string, confirm bool, featureRequest string) {
	// Prevent disk pressure on smaller systems due to leaking temp files
	_ = os.RemoveAll("/tmp/zarf*")
	tempPath := createPaths()

	if utils.InvalidPath(packageName) {
		logrus.WithField("archive", packageName).Fatal("The package archive seems to be missing or unreadable.")
	}

	logrus.Info("Extracting the package, this may take a few moments")

	// Extract the archive
	err := archiver.Unarchive(packageName, tempPath.base)
	if err != nil {
		logrus.Fatal("Unable to extract the package contents")
	}

	configPath := tempPath.base + "/zarf-config.yaml"
	confirm = confirmAction(configPath, confirm, "Deploy")

	// Don't continue unless the user says so
	if !confirm {
		cleanup(tempPath)
		os.Exit(0)
	}

	// Load the config from the extracted archive zarf-config.yaml
	config.DynamicConfigLoad(tempPath.base + "/zarf-config.yaml")

	dataInjectionList := config.GetDataInjections()
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
		if len(dataInjectionList) > 0 {
			logrus.Info("Loading data injections")
			injectionCompletionMarker := tempPath.dataInjections + "/.zarf-sync-complete"
			utils.WriteFile(injectionCompletionMarker, []byte("🦄"))
			for _, data := range dataInjectionList {
				sourceFile := tempPath.dataInjections + "/" + filepath.Base(data.Target.Path)
				pods := k8s.WaitForPodsAndContainers(data.Target)

				for _, pod := range pods {
					destination := data.Target.Path
					if destination == "/"+filepath.Base(destination) {
						// Handle top-level directory targets
						destination = "/"
					}
					cpPodExecArgs := []string{"kubectl", "-n", data.Target.Namespace, "cp", sourceFile, pod + ":" + destination}

					if data.Target.Container != "" {
						// Append the container args if they are specified
						cpPodExecArgs = append(cpPodExecArgs, "-c", data.Target.Container)
					}

					_, err = utils.ExecCommand(nil, config.K3sBinary, cpPodExecArgs...)
					if err != nil {
						logrus.Warn("Error copying data into the pod")
					} else {
						// Leave a marker in the target container for pods to track the sync action
						cpPodExecArgs[4] = injectionCompletionMarker
						cpPodExecArgs[5] = pod + ":" + data.Target.Path
						_, err = utils.ExecCommand(nil, config.K3sBinary, cpPodExecArgs...)
						if err != nil {
							logrus.Warn("Error saving the zarf sync completion file")
						}
					}
				}
				// Cleanup now to reduce disk pressure
				_ = os.RemoveAll(sourceFile)
			}
		}

		if len(remoteImageList) > 0 {
			logrus.Info("Loading images for remote install")
			// Push all images the images.tar file based on the zarf-config.yaml list
			images.PushAll(tempPath.remoteImage, remoteImageList, config.ZarfLocal)
			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(tempPath.remoteImage)
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
			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(sourceFile)
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
			_, err := utils.ExecCommand(nil, config.K3sBinary, "ctr", "images", "import", tempPath.localImage)
			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(tempPath.localImage)
			if err != nil {
				logrus.Fatal("Unable to import the images into containerd")
			}
		}
	}

	if assets.Manifests != "" {
		logrus.Info("Loading manifests for local install, this may take a minute or so to reflect in k3s")

		gitSecret := git.GetOrCreateZarfSecret()

		// Get a list of all the k3s manifest files
		manifests := utils.RecursiveFileList(tempPath.localManifests)

		// Iterate through all the manifests and replace any ZARF_SECRET values
		for _, manifest := range manifests {
			logrus.WithField("path", manifest).Info("Processing manifest file")
			utils.ReplaceText(manifest, "###ZARF_SECRET###", gitSecret)
		}

		utils.CreatePathAndCopy(tempPath.localManifests, config.K3sManifestPath)
	}
}
