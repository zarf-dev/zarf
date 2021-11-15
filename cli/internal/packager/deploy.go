package packager

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

func Deploy(packagePath string, confirm bool, componentRequest string) {
	// Prevent disk pressure on smaller systems due to leaking temp files
	_ = os.RemoveAll("/tmp/zarf*")
	tempPath := createPaths()

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(packagePath) {
		logrus.WithField("localPackagePath", packagePath).Fatal("Was not able to find the package on the local system")
	}

	// Extract the archive
	logrus.Info("Extracting the package, this may take a few moments")
	err := archiver.Unarchive(packagePath, tempPath.base)
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
	var requestedComponents []string
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
	if component.Name != "" {
		// Only log this for named components
		logrus.WithField("name", component.Name).Info("Deploying Zarf component")
	} else {
		component.Name = "core"
	}

	for _, script := range component.Scripts.Before {
		loopScriptUntilSuccess(script)
	}

	for index, file := range component.Files {
		sourceFile := tempPath.files + "/" + strconv.Itoa(index)

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			utils.ValidateSha256Sum(file.Shasum, sourceFile)
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

	if len(component.Appliance.Images) > 0 {
		// Handle appliance mode images
		logrus.Info("Loading images for appliance mode install")
		images.PushAll(tempPath.imagesAppliance, component.Appliance.Images, config.ZarfLocalIP+":45000")
		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(tempPath.imagesAppliance)
	}

	for _, chart := range component.Appliance.Charts {
		// Create chart temp path
		targetBase := helm.StandardName(tempPath.base+"/generated-charts", chart) + "/"

		// Generate helm templates to pass to gitops engine
		templatedChart := helm.TemplateChart(helm.ChartOptions{
			BasePath: tempPath.base,
			Chart:    chart,
		})

		// Save the manifest contents to disk
		utils.CreateFilePath(targetBase)
		utils.WriteFile(targetBase+"template.yaml", []byte(templatedChart))

		injectManifestSecrets(targetBase)
		k8s.GitopsProcess(targetBase, time.Now().Format(time.RFC3339Nano), chart.Namespace)
	}

	if component.Appliance.ManifestsPath != "" {
		logrus.Info("Loading manifests for local install")
		injectManifestSecrets(tempPath.manifests)
		k8s.GitopsProcess(tempPath.manifests, time.Now().Format(time.RFC3339Nano), "")
	}

	if len(component.Gitops.Images) > 0 {
		// Handle gitops images
		logrus.Info("Sending images to the gitops service registry")
		images.PushAll(tempPath.imagesGitops, component.Appliance.Images, config.ZarfLocalIP)
		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(tempPath.imagesGitops)
	}

	if len(component.Gitops.Repos) > 0 {
		logrus.Info("Loading git repos for gitops service transfer")
		// Push all the repos from the extracted archive
		git.PushAllDirectories(tempPath.repos)
	}

	for _, script := range component.Scripts.After {
		loopScriptUntilSuccess(script)
	}

}

// HandleIfURL If provided package is a URL download it to a temp directory
func HandleIfURL(packagePath string, shasum string, insecureDeploy bool) string {
	// Check if the user gave us a remote package
	providedURL, err := url.Parse(packagePath)
	if err != nil || providedURL.Scheme == "" || providedURL.Host == "" {
		logrus.WithField("archive", packagePath).Debug("The package provided is not a remote package.")
		return packagePath
	}

	if !insecureDeploy && shasum == "" {
		logrus.Fatal("When deploying a remote package you must provide either a `--shasum` or the `--insecure` flag. Neither were provided.")
	}

	// Check the extension on the package is what we expect
	if !isValidFileExtension(providedURL.Path) {
		logrus.Fatalf("Only %s file extensions are permitted.\n", config.GetValidPackageExtensions)
	}

	// Download the package
	resp, err := http.Get(packagePath)
	if err != nil {
		logrus.Fatal("Unable to download the package: ", err)
	}
	defer resp.Body.Close()

	// Write the package to a local file
	tempPath := createPaths()
	localPackagePath := tempPath.base + providedURL.Path
	logrus.Debug("Creating local package with the path: ", localPackagePath)
	packageFile, _ := os.Create(localPackagePath)
	_, err = io.Copy(packageFile, resp.Body)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to copy the contents of the provided URL into a local file.")
	}

	// Check the shasum if necessary
	if !insecureDeploy {
		hasher := sha256.New()
		_, err = io.Copy(hasher, packageFile)
		if err != nil {
			logrus.Debug(err)
			logrus.Fatal("Unable to calculate the sha256 of the provided remote package.")
		}

		value := hex.EncodeToString(hasher.Sum(nil))
		if value != shasum {
			_ = os.Remove(localPackagePath)
			logrus.Fatalf("Provided shasum (%s) of the package did not match what was downloaded (%s)\n", shasum, value)
		}
	}

	return localPackagePath
}

func isValidFileExtension(filename string) bool {
	for _, extension := range config.GetValidPackageExtensions() {
		if strings.HasSuffix(filename, extension) {
			logrus.WithField("packagePath", filename).Warn("Package extension is valid.")
			return true
		}
	}

	return false
}

func injectManifestSecrets(path string) {
	gitSecret := git.GetOrCreateZarfSecret()

	zarfHtPassword, err := utils.GetHtpasswdString(config.ZarfGitUser, gitSecret)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to define `htpasswd` string for the Zarf user")
	}
	zarfDockerAuth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", config.ZarfGitUser, gitSecret)))

	// Get a list of all the k3s manifest files
	manifests := utils.RecursiveFileList(path)

	// Iterate through all the manifests and replace any ZARF_SECRET, ZARF_HTPASSWD, or ZARF_DOCKERAUTH values
	for _, manifest := range manifests {
		logrus.WithField("path", manifest).Info("Processing manifest file")
		utils.ReplaceText(manifest, "###ZARF_SECRET###", gitSecret)
		utils.ReplaceText(manifest, "###ZARF_HTPASSWD###", zarfHtPassword)
		utils.ReplaceText(manifest, "###ZARF_DOCKERAUTH###", zarfDockerAuth)
	}
}

func loopScriptUntilSuccess(script string) {
	logContext := logrus.WithField("script", script)
	logContext.Info("Waiting for script to complete successfully")

	var output string
	var err error

	// 2 minutes per script (60 * 2 second waits)
	tries := 60
	for {
		tries--
		// If there are no more tries left, drop a warning and continue
		if tries < 1 {
			logContext.Warn("Script timed out after 2 minutes")
			logContext.Print(output)
			break
		}
		// Try to silently run the script
		output, err = utils.ExecCommand(false, nil, "sh", "-c", script)
		if err != nil {
			// On error, wait 2 seconds and try again
			logrus.Debug(err)
			time.Sleep(time.Second * 2)
			continue
		} else {
			// Script successful, output results and continue
			logContext.Print(output)
			logContext.Info("Script completed successfully")
			break
		}
	}
}
