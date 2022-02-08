package packager

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/git"
	"github.com/defenseunicorns/zarf/cli/internal/helm"
	"github.com/defenseunicorns/zarf/cli/internal/images"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/pki"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

func Deploy(packagePath string, confirm bool, componentRequest string) {
	// Prevent disk pressure on smaller systems due to leaking temp files
	_ = os.RemoveAll("/tmp/zarf*")
	tempPath := createPaths()

	logContext := logrus.WithFields(logrus.Fields{
		"path":       packagePath,
		"confirm":    confirm,
		"components": componentRequest,
	})

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(packagePath) {
		logContext.Fatal("Was not able to find the package on the local system")
	}

	// Extract the archive
	logContext.Info("Extracting the package, this may take a few moments")
	err := archiver.Unarchive(packagePath, tempPath.base)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to extract the package contents")
	}

	configPath := tempPath.base + "/zarf.yaml"
	confirm = confirmAction(configPath, confirm, "Deploy")

	// Don't continue unless the user says so
	if !confirm {
		cleanup(tempPath)
		os.Exit(0)
	}

	// Load the config from the extracted archive zarf.yaml
	if err := config.LoadConfig(tempPath.base + "/zarf.yaml"); err != nil {
		logContext.Debug(err)
		logContext.Fatalf("Unable to read the zarf.yaml file from %s", tempPath.base)
	}

	dataInjectionList := config.GetDataInjections()

	// Verify the components requested all exist
	components := config.GetComponents()
	var requestedComponents []string
	if componentRequest != "" {
		requestedComponents = strings.Split(componentRequest, ",")
	}
	componentsToDeploy := getValidComponents(components, requestedComponents)

	// Deploy all of the components
	for _, component := range componentsToDeploy {
		componentPath := createComponentPaths(tempPath.components, component)
		deployComponents(componentPath, component)
	}

	if !config.IsZarfInitConfig() {
		if len(dataInjectionList) > 0 {
			logContext.Info("Loading data injections")
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

					_, err = utils.ExecCommand(true, nil, config.K3sBinary, cpPodExecArgs...)
					if err != nil {
						logrus.Warn("Error copying data into the pod")
					} else {
						// Leave a marker in the target container for pods to track the sync action
						cpPodExecArgs[4] = injectionCompletionMarker
						cpPodExecArgs[5] = pod + ":" + data.Target.Path
						_, err = utils.ExecCommand(true, nil, config.K3sBinary, cpPodExecArgs...)
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

func deployComponents(tempPath componentPaths, component config.ZarfComponent) {
	values := generateTemplateValues()

	if component.Name != "" {
		// Only log this for named components
		logrus.WithField("name", component.Name).Info("Deploying Zarf component")
	} else {
		component.Name = "core"
	}

	for _, script := range component.Scripts.Before {
		loopScriptUntilSuccess(script, component.Scripts.Retry)
	}

	for index, file := range component.Files {
		sourceFile := tempPath.files + "/" + strconv.Itoa(index)

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			utils.ValidateSha256Sum(file.Shasum, sourceFile)
		}

		// Perform secret injection if the file is marked as template
		if file.Template {
			templateFile(sourceFile, values)
		}

		// Copy the file to the destination
		err := copy.Copy(sourceFile, file.Target)
		if err != nil {
			logrus.Debug(err)
			logrus.WithField("file", file.Target).Fatal("Unable to copy the contents of the asset")
		}

		for _, link := range file.Symlinks {
			// Try to remove the filepath if it exists
			_ = os.RemoveAll(link)
			// Make sure the parent directory exists
			utils.CreateFilePath(link)
			// Create the symlink
			err := os.Symlink(file.Target, link)
			if err != nil {
				logrus.Debug(err)
				logrus.WithField("target", link).Fatal("Unable to create the symbolic link")
			}
		}

		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(sourceFile)
	}

	if len(component.Charts) > 0 {
		logrus.Info("Loading charts for local install")
		for _, chart := range component.Charts {
			sourceTarball := helm.StandardName(tempPath.charts, chart)
			destinationTarball := helm.StandardName(config.K3sChartPath, chart)
			utils.CreatePathAndCopy(sourceTarball, destinationTarball)
		}
	}

	if len(component.Images) > 0 {
		logrus.Info("Loading images for local install")
		if config.IsZarfInitConfig() {
			_, err := utils.ExecCommand(true, nil, config.K3sBinary, "ctr", "images", "import", tempPath.images)
			if err != nil {
				logrus.Fatal("Unable to import the images into containerd")
			}
		} else {
			logrus.Info("Loading images for gitops service transfer")
			// Push all images the images.tar file based on the zarf.yaml list
			images.PushAll(tempPath.images, component.Images, config.GetTargetEndpoint())
			// Cleanup now to reduce disk pressure
			_ = os.RemoveAll(tempPath.images)
		}
	}

	if component.ManifestsPath != "" {
		logrus.Info("Loading manifests for local install, this may take a minute or so to reflect in k3s")

		// Only pull in yml and yaml files
		pattern := regexp.MustCompile(`(?mi)\.ya?ml$`)
		manifests := utils.RecursiveFileList(tempPath.manifests, pattern)

		// Iterate through all the manifests and replace any ZARF_SECRET, ZARF_HTPASSWD, or ZARF_DOCKERAUTH values
		for _, manifest := range manifests {
			templateFile(manifest, values)
		}

		utils.CreatePathAndCopy(tempPath.manifests, config.K3sManifestPath)
	}

	if len(component.Repos) > 0 {
		logrus.Info("Loading git repos for gitops service transfer")
		// Push all the repos from the extracted archive
		git.PushAllDirectories(tempPath.repos)
	}

	for _, script := range component.Scripts.After {
		loopScriptUntilSuccess(script, component.Scripts.Retry)
	}

	if config.IsZarfInitConfig() && component.Name == "k3s" {
		pki.InjectServerCert()
	}

}

type templateValues struct {
	secret     string
	htpasswd   string
	dockerAuth string
	endpoint   string
}

func generateTemplateValues() templateValues {
	var generated templateValues
	var err error

	generated.secret = git.GetOrCreateZarfSecret()
	generated.htpasswd, err = utils.GetHtpasswdString(config.ZarfGitUser, generated.secret)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to define `htpasswd` string for the Zarf user")
	}
	generated.dockerAuth = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", config.ZarfGitUser, generated.secret)))
	generated.endpoint = config.GetTargetEndpoint()
	return generated
}

func templateFile(path string, values templateValues) {
	logrus.WithField("path", path).Info("Processing file for templating")
	utils.ReplaceText(path, "###ZARF_TARGET_ENDPOINT###", values.endpoint)
	utils.ReplaceText(path, "###ZARF_SECRET###", values.secret)
	utils.ReplaceText(path, "###ZARF_HTPASSWD###", values.htpasswd)
	utils.ReplaceText(path, "###ZARF_DOCKERAUTH###", values.dockerAuth)
}
