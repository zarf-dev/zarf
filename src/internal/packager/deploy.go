package packager

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/images"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/template"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/pterm/pterm"
)

var valueTemplate template.Values
var connectStrings = make(types.ConnectStrings)

// Deploy attempts to deploy a Zarf package that is define within the global DeployOptions struct
func Deploy() {
	message.Debug("packager.Deploy()")

	tempPath := createPaths()
	defer tempPath.clean()

	spinner := message.NewProgressSpinner("Preparing zarf package %s", config.DeployOptions.PackagePath)
	defer spinner.Stop()

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(config.DeployOptions.PackagePath) {
		spinner.Fatalf(nil, "Unable to find the package on the local system, expected package at %s", config.DeployOptions.PackagePath)
	}

	// Extract the archive
	spinner.Updatef("Extracting the package, this may take a few moments")
	err := archiver.Unarchive(config.DeployOptions.PackagePath, tempPath.base)
	if err != nil {
		spinner.Fatalf(err, "Unable to extract the package contents")
	}

	// Load the config from the extracted archive zarf.yaml
	spinner.Updatef("Loading the zarf package config")
	configPath := filepath.Join(tempPath.base, "zarf.yaml")
	if err := config.LoadConfig(configPath, true); err != nil {
		spinner.Fatalf(err, "Invalid or unreadable zarf.yaml file in %s", tempPath.base)
	}

	if config.IsZarfInitConfig() {
		// If init config, make sure things are ready
		utils.RunPreflightChecks()
	}

	spinner.Success()

	sbomViewFiles, _ := filepath.Glob(tempPath.sboms + "/sbom-viewer-*")
	// If SBOM files exist, temporary place them in the deploy directory
	if len(sbomViewFiles) > 0 {
		sbomDir := "zarf-sbom"
		// Cleanup any failed prior removals
		_ = os.RemoveAll(sbomDir)
		// Create the directory again
		utils.CreateDirectory(sbomDir, 0755)
		for _, file := range sbomViewFiles {
			// Our file copy lib explodes on these files for some reason...
			data, err := ioutil.ReadFile(file)
			if err != nil {
				message.Fatalf(err, "Unable to read the sbom-viewer file %s", file)
			}
			dst := filepath.Join(sbomDir, filepath.Base(file))
			err = ioutil.WriteFile(dst, data, 0644)
			if err != nil {
				message.Fatalf(err, "Unable to write the sbom-viewer file %s", dst)
			}
		}
	}

	// Confirm the overall package deployment
	confirm := confirmAction("Deploy", sbomViewFiles)

	// Don't continue unless the user says so
	if !confirm {
		os.Exit(0)
	}

	// Set variables and prompt if --confirm is not set
	if err := config.SetActiveVariables(); err != nil {
		message.Fatalf(err, "Unable to set variables in config: %s", err.Error())
	}

	// Verify the components requested all exist
	components := config.GetComponents()
	var requestedComponents []string
	if config.DeployOptions.Components != "" {
		requestedComponents = strings.Split(config.DeployOptions.Components, ",")
	}

	// Get a list of all the components we are deploying and actually deploy them
	componentsToDeploy := getValidComponents(components, requestedComponents)
	deployComponents(tempPath, componentsToDeploy)

	message.SuccessF("Zarf deployment complete")
	pterm.Println()

	// If not init config, print the application connection table
	if !config.IsZarfInitConfig() {
		message.PrintConnectStringTable(connectStrings)
	} else {
		// otherwise, print the init config connection and passwords
		loginTableHeader := pterm.TableData{
			{"     Application", "Username", "Password", "Connect"},
		}

		loginTable := pterm.TableData{}
		if config.GetContainerRegistryInfo().InternalRegistry {
			loginTable = append(loginTable, pterm.TableData{{"     Registry", config.GetContainerRegistryInfo().PushUsername, config.GetContainerRegistryInfo().PushPassword, "zarf connect registry"}}...)
		}

		for _, component := range componentsToDeploy {
			// Show message if including logging stack
			if component.Name == "logging" {
				loginTable = append(loginTable, pterm.TableData{{"     Logging", "zarf-admin", config.GetState().LoggingSecret, "zarf connect logging"}}...)
			}
			// Show message if including git-server
			if component.Name == "git-server" {
				loginTable = append(loginTable, pterm.TableData{
					{"     Git", config.GetGitServerInfo().PushUsername, config.GetState().GitServer.PushPassword, "zarf connect git"},
					{"     Git (read-only)", config.GetGitServerInfo().PullUsername, config.GetState().GitServer.PullPassword, "zarf connect git"},
				}...)
			}
		}

		if len(loginTable) > 0 {
			loginTable = append(loginTableHeader, loginTable...)
			_ = pterm.DefaultTable.WithHasHeader().WithData(loginTable).Render()
		}
	}

	// All done
	os.Exit(0)
}

// deployComponents loops through a list of ZarfComponents and deploys them
func deployComponents(tempPath tempPaths, componentsToDeploy []types.ZarfComponent) {
	// When pushing images, the default behavior is to add a shasum of the url to the image name
	addShasumToImg := true

	// Deploy all the components
	for _, component := range componentsToDeploy {

		// If this is an init-package and we are using an external registry, don't deploy the components to stand up an internal registry
		// TODO: Figure out a better way to do this (I don't like how these components are still `required` according to the yaml definition)
		if (config.IsZarfInitConfig() && config.InitOptions.RegistryInfo.Address != "") &&
			(component.Name == "zarf-seed-registry" || component.Name == "zarf-injector" || component.Name == "zarf-registry") {
			message.Notef("Not deploying the component (%s) since external registry information was provided during `zarf init`", component.Name)
			continue
		}

		// Do somewhat custom deploys for the seed and agent components
		switch component.Name {
		case "zarf-seed-registry":
			// If this is the seed component; seed the state and inject a container registry into the cluster
			if component.Name == "zarf-seed-registry" && config.InitOptions.RegistryInfo.Address == "" {
				seedZarfState(tempPath)
				runInjectionMadness(tempPath)
				deployComponent(tempPath, component, !addShasumToImg)
				postSeedRegistry(tempPath)
			}
		case "zarf-agent":
			// Seed the state if we are using an external registry
			if !config.GetContainerRegistryInfo().InternalRegistry {
				seedZarfState(tempPath)
			}
			deployComponent(tempPath, component, !addShasumToImg)
		default:
			deployComponent(tempPath, component, addShasumToImg)
		}
	}
}

func deployComponent(tempPath tempPaths, component types.ZarfComponent, addShasumToImgs bool) {
	message.Debugf("packager.deployComponents(%#v, %#v", tempPath, component)

	hasImages := len(component.Images) > 0
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasDataInjections := len(component.DataInjections) > 0

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))
	componentPath := createComponentPaths(tempPath.components, component)

	// Run the 'before' scripts and move files before we do anything else
	runComponentScripts(component.Scripts.Before, component.Scripts)
	processComponentFiles(component.Files, componentPath.files, tempPath.base)

	// Generate a value template
	valueTemplate = template.Generate()
	if !valueTemplate.Ready() && (hasImages || hasCharts || hasManifests || hasRepos) {
		valueTemplate = getUpdatedValueTemplate(valueTemplate, component)
	}

	/* Install all the parts of the component */
	if hasImages {
		pushImagesToRegistry(tempPath, component.Images, addShasumToImgs)
	}

	if hasRepos {
		pushReposToRepository(componentPath.repos, component.Repos)
	}

	if hasDataInjections {
		waitGroup := sync.WaitGroup{}
		defer waitGroup.Wait()
		performDataInjections(&waitGroup, componentPath, component.DataInjections)
	}

	if hasCharts || hasManifests {
		installChartAndManifests(componentPath, component)
	}

	// Run the 'after' scripts after all other attributes of the component has been deployed
	runComponentScripts(component.Scripts.After, component.Scripts)

}

// Run scripts that a component has provided
func runComponentScripts(scripts []string, componentScript types.ZarfComponentScripts) {
	for _, script := range scripts {
		loopScriptUntilSuccess(script, componentScript)
	}
	return
}

// Move files onto the host of the machine performing the deployment
func processComponentFiles(componentFiles []types.ZarfFile, sourceLocation, tempPathBase string) {
	var spinner message.Spinner
	if len(componentFiles) > 0 {
		spinner = *message.NewProgressSpinner("Copying %d files", len(componentFiles))
		defer spinner.Stop()
	}

	for index, file := range componentFiles {
		spinner.Updatef("Loading %s", file.Target)
		// sourceFile := componentPath.files + "/" + strconv.Itoa(index)
		sourceFile := sourceLocation + "/" + strconv.Itoa(index)

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			spinner.Updatef("Validating SHASUM for %s", file.Target)
			utils.ValidateSha256Sum(file.Shasum, sourceFile)
		}

		// Replace temp target directories
		file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", tempPathBase, 1)

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
			_ = utils.CreateFilePath(link)
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

}

// Fetch the current ZarfState from the k8s cluster and generate a valueTemplate from the state values
func getUpdatedValueTemplate(valueTemplate template.Values, component types.ZarfComponent) template.Values {
	// If we are touching K8s, make sure we can talk to it once per deployment
	spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
	defer spinner.Stop()

	state := k8s.LoadZarfState()

	if state.Distro == "" {
		// If no distro the zarf secret did not load properly
		spinner.Fatalf(nil, "Unable to load the zarf/zarf-state secret, did you remember to run zarf init first?")
	}

	// Continue loading state data if it is valid
	config.InitState(state)
	valueTemplate = template.Generate()
	if len(component.Images) > 0 && state.Architecture != config.GetArch() {
		// If the package has images but the architectures don't match warn the user to avoid ugly hidden errors with image push/pull
		spinner.Fatalf(nil, "This package architecture is %s, but this cluster seems to be initialized with the %s architecture",
			config.GetArch(),
			state.Architecture)
	}

	spinner.Success()

	return valueTemplate
}

// Push all of the components images to the configured container registry
func pushImagesToRegistry(tempPath tempPaths, componentImages []string, addShasumToImg bool) {
	if len(componentImages) == 0 {
		return
	}

	// Try image push up to 3 times
	for retry := 0; retry < 3; retry++ {
		if err := images.PushToZarfRegistry(tempPath.images, componentImages, addShasumToImg); err != nil {
			message.Errorf(err, "Unable to push images to the Registry, retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
}

// Push all of the components git repos to the configured git server
func pushReposToRepository(reposPath string, repos []string) {
	if len(repos) == 0 {
		return
	}

	// Try repo push up to 3 times
	for retry := 0; retry < 3; retry++ {
		// Push all the repos from the extracted archive
		if err := git.PushAllDirectories(reposPath); err != nil {
			message.Errorf(err, "Unable to push repos to the Git Server, retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
}

// Async'ly move data into a container running in a pod on the k8s cluster
func performDataInjections(waitGroup *sync.WaitGroup, componentPath componentPaths, dataInjections []types.ZarfDataInjection) {
	if len(dataInjections) > 0 {
		message.Info("Loading data injections")
	}

	for _, data := range dataInjections {
		waitGroup.Add(1)
		go handleDataInjection(waitGroup, data, componentPath)
	}
}

// Install all Helm charts and raw k8s manifests into the k8s cluster
func installChartAndManifests(componentPath componentPaths, component types.ZarfComponent) {
	for _, chart := range component.Charts {
		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			chartValueName := helm.StandardName(componentPath.values, chart) + "-" + strconv.Itoa(idx)
			valueTemplate.Apply(component, chartValueName)
		}

		// Generate helm templates to pass to gitops engine
		addedConnectStrings := helm.InstallOrUpgradeChart(helm.ChartOptions{
			BasePath:  componentPath.base,
			Chart:     chart,
			Component: component,
		})

		// Iterate over any connectStrings and add to the main map
		for name, description := range addedConnectStrings {
			connectStrings[name] = description
		}
	}

	for _, manifest := range component.Manifests {
		for idx := range manifest.Kustomizations {
			// Move kustomizations to files now
			destination := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, idx)
			manifest.Files = append(manifest.Files, destination)
		}

		// Iterate over any connectStrings and add to the main map
		for name, description := range helm.GenerateChart(componentPath.manifests, manifest, component) {
			connectStrings[name] = description
		}
	}
}
