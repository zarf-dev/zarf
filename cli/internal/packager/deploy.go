package packager

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/git"
	"github.com/defenseunicorns/zarf/cli/internal/helm"
	"github.com/defenseunicorns/zarf/cli/internal/images"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

func Deploy(packageName string, confirm bool, componentRequest string) {
	// Prevent disk pressure on smaller systems due to leaking temp files
	_ = os.RemoveAll("/tmp/zarf*")
	tempPath := createPaths()

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(packageName) {
		logrus.WithField("packageName", packageName).Debug("Was not able to find the package on the local system. Trying to process as a URL now.")

		// Check if the user gave us a remote package
		providedURL, err := url.Parse(packageName)
		if err != nil || providedURL.Scheme == "" || providedURL.Host == "" {
			logrus.WithField("archive", packageName).Fatal("The package archive seems to be missing or unreadable.")
		}

		// TODO @JPERRY Do we want to do any other verification checks before download? (in addition to the extension check)
		// Check the extension on the package is what we expect
		if !isValidFileExtension(providedURL.Path) {
			logrus.Fatal("The URL provided had an unrecognized file extension.")
		}

		// Download the package
		// TODO @JPERRY Should we handle potential credentials to access the package?
		resp, err := http.Get(packageName)
		if err != nil {
			logrus.Fatal("Unable to download the package: ", err)
		}
		defer resp.Body.Close()

		// Write the package to a local file
		packageName = providedURL.Path[1:] // Remove the leading slash
		logrus.Debug("Creating local package with the name: ", packageName)
		outfile, _ := os.Create(packageName)
		defer os.Remove(outfile.Name()) // TODO @JPERRY removes after successful deploy but not if you say 'no' to the prompt or if logrus.Fatal() is called
		_, err = io.Copy(outfile, resp.Body)
		if err != nil {
			logrus.Debug(err)
			logrus.Fatal("Unable to copy the contents of the provided URL into a local file.")
		}
	}

	logrus.Info("Extracting the package, this may take a few moments")
	// Extract the archive
	err := archiver.Unarchive(packageName, tempPath.base)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to extract the package contents")
	}

	configPath := tempPath.base + "/zarf.yaml"
	confirm = confirmAction(configPath, confirm, "Deploy")

	// Don't continue unless the user says so
	if !confirm {
		cleanup(tempPath)
		os.Exit(0)
	}

	// Load the config from the extracted archive zarf.yaml
	config.Load(tempPath.base + "/zarf.yaml")

	dataInjectionList := config.GetDataInjections()

	// Verify the components requested all exist
	components := config.GetComponents()
	requestedComponents := []string{}
	if componentRequest != "" {
		requestedComponents = strings.Split(componentRequest, ",")
	}
	componentsToDeploy := utils.GetValidComponents(components, requestedComponents)

	// Deploy all of the components
	for _, component := range componentsToDeploy {
		componentPath := createComponentPaths(tempPath.components, component)
		deployComponents(componentPath, component)
	}

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

	}

	cleanup(tempPath)
}

func deployComponents(tempPath componentPaths, assets config.ZarfComponent) {
	if assets.Name != "" {
		// Only log this for named components
		logrus.WithField("name", assets.Name).Info("Deploying Zarf component")
	} else {
		assets.Name = "core"
	}
	if len(assets.Files) > 0 {
		logrus.Info("Loading files for local install")
		for index, file := range assets.Files {
			sourceFile := tempPath.files + "/" + strconv.Itoa(index)
			// If a shasum is specified check it again on deployment as well
			if file.Shasum != "" {
				utils.ValidateSha256Sum(file.Shasum, sourceFile)
			}
			err := copy.Copy(sourceFile, file.Target)
			if err != nil {
				logrus.Debug(err)
				logrus.WithField("file", file.Target).Fatal("Unable to copy the contents of the asset")
			}
			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(sourceFile)
		}
	}

	if len(assets.Charts) > 0 {
		logrus.Info("Loading charts for local install")
		for _, chart := range assets.Charts {
			sourceTarball := helm.StandardName(tempPath.charts, chart)
			destinationTarball := helm.StandardName(config.K3sChartPath, chart)
			utils.CreatePathAndCopy(sourceTarball, destinationTarball)
		}
	}

	if len(assets.Images) > 0 {
		logrus.Info("Loading images for local install")
		if config.IsZarfInitConfig() {
			utils.CreatePathAndCopy(tempPath.images, config.K3sImagePath+"/images-"+assets.Name+".tar")
		} else {
			logrus.Info("Loading images for gitops service transfer")
			// Push all images the images.tar file based on the zarf.yaml list
			images.PushAll(tempPath.images, assets.Images)
			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(tempPath.images)
		}
	}

	if assets.Manifests != "" {
		logrus.Info("Loading manifests for local install, this may take a minute or so to reflect in k3s")

		gitSecret := git.GetOrCreateZarfSecret()

		// Get a list of all the k3s manifest files
		manifests := utils.RecursiveFileList(tempPath.manifests)

		// Iterate through all the manifests and replace any ZARF_SECRET values
		for _, manifest := range manifests {
			logrus.WithField("path", manifest).Info("Processing manifest file")
			utils.ReplaceText(manifest, "###ZARF_SECRET###", gitSecret)
		}

		utils.CreatePathAndCopy(tempPath.manifests, config.K3sManifestPath)
	}

	if len(assets.Repos) > 0 {
		logrus.Info("Loading git repos for gitops service transfer")
		// Push all the repos from the extracted archive
		git.PushAllDirectories(tempPath.repos)
	}
}

func isValidFileExtension(filename string) bool {
	// TODO @JPERRY What other extensions do we want to allow?
	validExtensions := []string{".tar.zst", ".tar", "zip"}

	for _, extension := range validExtensions {
		if strings.HasSuffix(filename, extension) {
			logrus.WithField("packageName", filename).Warn("Package extension is valid.")
			return true
		}
	}

	return false
}
