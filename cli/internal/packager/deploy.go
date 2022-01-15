package packager

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/git"
	"github.com/defenseunicorns/zarf/cli/internal/helm"
	"github.com/defenseunicorns/zarf/cli/internal/images"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/template"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
)

var valueTemplate template.Values

func Deploy() {
	message.Debug("packager.Deploy()")
	// Prevent disk pressure on smaller systems due to leaking temp files
	_ = os.RemoveAll("/tmp/zarf*")
	tempPath := createPaths()

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(config.DeployOptions.PackagePath) {
		message.Fatalf(nil, "Unable to find the package on the local system, expected package at %s", config.DeployOptions.PackagePath)
	}

	// Extract the archive
	message.Info("Extracting the package, this may take a few moments")
	err := archiver.Unarchive(config.DeployOptions.PackagePath, tempPath.base)
	if err != nil {
		message.Fatal(err, "Unable to extract the package contents")
	}

	// Load the config from the extracted archive zarf.yaml
	if err := config.LoadConfig(tempPath.base + "/zarf.yaml"); err != nil {
		message.Fatalf(err, "Unable to read the zarf.yaml file from %s", tempPath.base)
	}

	// Init config already verifies installation at the start
	if !config.IsZarfInitConfig() {
		configPath := tempPath.base + "/zarf.yaml"
		confirm := confirmAction(configPath, "Deploy")

		// Don't continue unless the user says so
		if !confirm {
			cleanup(tempPath)
			os.Exit(0)
		}
	}

	// Verify the components requested all exist
	components := config.GetComponents()
	var requestedComponents []string
	if config.DeployOptions.Components != "" {
		requestedComponents = strings.Split(config.DeployOptions.Components, ",")
	}
	componentsToDeploy := getValidComponents(components, requestedComponents)

	// Deploy all the components
	for _, component := range componentsToDeploy {
		deployComponents(tempPath, component)
	}

	if !config.IsZarfInitConfig() {
		dataInjectionList := config.GetDataInjections()
		if len(dataInjectionList) > 0 {
			message.Info("Loading data injections")
			handleDataInjection(dataInjectionList, tempPath)
		}

	}

	cleanup(tempPath)
}

func deployComponents(tempPath tempPaths, component config.ZarfComponent) {
	message.Debugf("packager.deployComponents(%v, %v", tempPath, component)
	componentPath := createComponentPaths(tempPath.components, component)
	isSeedRegistry := config.IsZarfInitConfig() && component.Name == "container-registry-seed"
	hasImages := len(component.Images) > 0
	hasCharts := len(component.Charts) > 0
	hasManifests := component.ManifestsPath != ""
	hasRepos := len(component.Repos) > 0

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	for _, script := range component.Scripts.Before {
		loopScriptUntilSuccess(script, component.Scripts.Retry)
	}

	spinner := message.NewProgressSpinner("Copying %v files", len(component.Files))
	defer spinner.Stop()

	for index, file := range component.Files {
		spinner.Updatef("Loading %s", file.Target)
		sourceFile := componentPath.files + "/" + strconv.Itoa(index)

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			spinner.Updatef("Validating SHASUM for %s", file.Target)
			utils.ValidateSha256Sum(file.Shasum, sourceFile)
		}

		// Copy the file to the destination
		spinner.Updatef("Saving %s", file.Target)
		err := copy.Copy(sourceFile, file.Target)
		if err != nil {
			spinner.Fatalf(err, "Unable to copy the contents of %s", file.Target)
		}

		// Loop over all symlinks and create them
		for _, link := range file.Symlinks {
			spinner.Updatef("Adding symlink %s->%s", link, file.Target)
			// Try to remove the filepath if it exists
			_ = os.RemoveAll(link)
			// Make sure the parent directory exists
			utils.CreateFilePath(link)
			// Create the symlink
			err := os.Symlink(file.Target, link)
			if err != nil {
				spinner.Fatalf(err, "Unable to create the symbolic link %s -> %s", link, file.Target)
			}
		}

		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(sourceFile)
	}
	spinner.Success()

	if isSeedRegistry {
		preSeedRegistry(tempPath)
		valueTemplate = template.Generate()
	}

	if !valueTemplate.Ready() && (hasImages || hasCharts || hasManifests || hasRepos) {
		spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
		defer spinner.Stop()

		state := k8s.LoadZarfState()
		config.InitState(state)
		valueTemplate = template.Generate()
		if state.Distro == "" {
			spinner.Fatalf(nil, "Unable to load the zarf/zarf-state secret")
		}
		spinner.Success()
	}

	if hasImages {
		images.PushAll(tempPath.images, component.Images)
	}

	for _, chart := range component.Charts {
		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			chartValueName := helm.StandardName(componentPath.values, chart) + "-" + strconv.Itoa(idx)
			valueTemplate.Apply(chartValueName)
		}

		// Generate helm templates to pass to gitops engine
		helm.InstallOrUpgradeChart(helm.ChartOptions{
			BasePath: componentPath.base,
			Chart:    chart,
			Images:   component.Images,
		})
	}

	if hasManifests {
		k8s.GitopsProcess(componentPath.manifests, "", component)
	}

	if hasRepos {
		// Push all the repos from the extracted archive
		git.PushAllDirectories(componentPath.repos)
	}

	for _, script := range component.Scripts.After {
		loopScriptUntilSuccess(script, component.Scripts.Retry)
	}

	if isSeedRegistry {
		postSeedRegistry()
	}
}

// handleDataInjection performs data-copy operations into a pod
// todo:  this currently requires kubectl but we should have enough k8s work to make this native now
func handleDataInjection(dataInjectionList []config.ZarfData, tempPath tempPaths) {
	injectionCompletionMarker := tempPath.dataInjections + "/.zarf-sync-complete"
	if err := utils.WriteFile(injectionCompletionMarker, []byte("🦄")); err != nil {
		return
	}
	for _, data := range dataInjectionList {
		sourceFile := tempPath.dataInjections + "/" + filepath.Base(data.Target.Path)
		pods := k8s.WaitForPodsAndContainers(data.Target, true)

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

			_, err := utils.ExecCommand(true, nil, config.K3sBinary, cpPodExecArgs...)
			if err != nil {
				message.Warn("Error copying data into the pod")
			} else {
				// Leave a marker in the target container for pods to track the sync action
				cpPodExecArgs[4] = injectionCompletionMarker
				cpPodExecArgs[5] = pod + ":" + data.Target.Path
				_, err = utils.ExecCommand(true, nil, config.K3sBinary, cpPodExecArgs...)
				if err != nil {
					message.Warn("Error saving the zarf sync completion file")
				}
			}
		}
		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(sourceFile)
	}
}
