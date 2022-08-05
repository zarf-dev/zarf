package packager

import (
	"encoding/json"
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
	corev1 "k8s.io/api/core/v1"
)

var valueTemplate template.Values
var connectStrings = make(types.ConnectStrings)

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

	// Generate a secret that describes the package that is being deployed
	secretName := fmt.Sprintf("zarf-package-%s", config.GetActiveConfig().Metadata.Name)
	deployedPackageSecret := k8s.GenerateSecret("zarf", secretName, corev1.SecretTypeOpaque)
	deployedPackageSecret.Labels["package-deploy-info"] = config.GetActiveConfig().Metadata.Name
	deployedPackageSecret.StringData = make(map[string]string)

	installedZarfPackage := types.DeployedPackage{
		Name:               config.GetActiveConfig().Metadata.Name,
		CLIVersion:         config.CLIVersion,
		Data:               config.GetActiveConfig(),
		DeployedComponents: make(map[string]types.DeployedComponent),
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
	componentsToDeploy := getValidComponents(components, requestedComponents)

	// Deploy all the components
	for _, component := range componentsToDeploy {
		installedCharts := deployComponents(tempPath, component)

		// Get information about what we just installed so we can save it to a secret later
		installedComponent := types.DeployedComponent{}
		if len(installedCharts) > 0 {
			installedComponent.InstalledCharts = installedCharts
		}

		installedZarfPackage.DeployedComponents[component.Name] = installedComponent
	}

	message.SuccessF("Zarf deployment complete")
	pterm.Println()

	// If not init config, print the application connection table
	if !config.IsZarfInitConfig() {
		message.PrintConnectStringTable(connectStrings)
	} else {
		// otherwise, print the init config connection and passwords
		loginTable := pterm.TableData{
			{"     Application", "Username", "Password", "Connect"},
			{"     Registry", config.GetContainerRegistryInfo().PushUser, config.GetContainerRegistryInfo().PushPassword, "zarf connect registry"},
		}
		for _, component := range componentsToDeploy {
			// Show message if including logging stack
			if component.Name == "logging" {
				loginTable = append(loginTable, pterm.TableData{{"     Logging", "zarf-admin", config.GetSecret(config.StateLogging), "zarf connect logging"}}...)
			}
			// Show message if including git-server
			if component.Name == "git-server" {
				loginTable = append(loginTable, pterm.TableData{
					{"     Git", config.GetGitServerInfo().PushUsername, config.GetState().GitServer.PushPassword, "zarf connect git"},
					{"     Git (read-only)", config.GetGitServerInfo().ReadUsername, config.GetState().GitServer.ReadPassword, "zarf connect git"},
				}...)
			}
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(loginTable).Render()
	}

	// Not all packages need k8s so we need to check if k8s is being used before saving the metadata secret
	if packageUsesK8s(config.GetActiveConfig()) {
		stateData, _ := json.Marshal(installedZarfPackage)
		deployedPackageSecret.Data = make(map[string][]byte)
		deployedPackageSecret.Data["data"] = stateData
		k8s.ReplaceSecret(deployedPackageSecret)
	}
}

func deployComponents(tempPath tempPaths, component types.ZarfComponent) []types.InstalledCharts {
	message.Debugf("packager.deployComponents(%#v, %#v", tempPath, component)

	var installedCharts []types.InstalledCharts

	// Toggles for deploy operations on 'init'
	isSeedRegistry := config.IsZarfInitConfig() && component.Name == "zarf-seed-registry"
	injectRegistry := isSeedRegistry && config.InitOptions.ContainerRegistryInfo.URL == ""
	doSeedThings := config.IsZarfInitConfig() && !config.GetContainerRegistryInfo().InternalRegistry && component.Name == "zarf-agent"

	// Toggles for general deploy operations
	componentPath := createComponentPaths(tempPath.components, component)
	hasImages := len(component.Images) > 0
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasDataInjections := len(component.DataInjections) > 0

	// Don't inject a registry if an external one has been provided
	// TODO: Figure out a better way to do this (I don't like how these components are still `required` according to the yaml definition)
	if (config.InitOptions.ContainerRegistryInfo.URL != "") && (component.Name == "zarf-injector" || component.Name == "zarf-seed-registry" || component.Name == "zarf-registry") {
		message.Notef("Not deploying the component (%s) since external registry information was provided during `zarf init`", component.Name)
		return installedCharts
	}

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))
	for _, script := range component.Scripts.Before {
		loopScriptUntilSuccess(script, component.Scripts)
	}

	if len(component.Files) > 0 {
		spinner := message.NewProgressSpinner("Copying %d files", len(component.Files))
		defer spinner.Stop()

		for index, file := range component.Files {
			spinner.Updatef("Loading %s", file.Target)
			sourceFile := componentPath.files + "/" + strconv.Itoa(index)

			// If a shasum is specified check it again on deployment as well
			if file.Shasum != "" {
				spinner.Updatef("Validating SHASUM for %s", file.Target)
				utils.ValidateSha256Sum(file.Shasum, sourceFile)
			}

			// Replace temp target directories
			file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", tempPath.base, 1)

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

	if isSeedRegistry || doSeedThings {
		preSeedRegistry(tempPath, injectRegistry)
		valueTemplate = template.Generate()
	}

	if !valueTemplate.Ready() && (hasImages || hasCharts || hasManifests || hasRepos) {
		// If we are touching K8s, make sure we can talk to it once per deployment
		spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
		defer spinner.Stop()

		state, err := k8s.LoadZarfState()
		if err != nil {
			spinner.Fatalf(err, "Unable to load the Zarf State from the Kubernetes cluster")
		}

		if state.Distro == "" {
			// If no distro the zarf secret did not load properly
			spinner.Fatalf(nil, "Unable to load the zarf/zarf-state secret, did you remember to run zarf init first?")
		}

		// Continue loading state data if it is valid
		config.InitState(state)
		valueTemplate = template.Generate()
		if hasImages && state.Architecture != config.GetArch() {
			// If the package has images but the architectures don't match warn the user to avoid ugly hidden errors with image push/pull
			spinner.Fatalf(nil, "This package architecture is %s, but this cluster seems to be initialized with the %s architecture",
				config.GetArch(),
				state.Architecture)
		}

		spinner.Success()
	}

	if hasImages {
		// Try image push up to 3 times
		for retry := 0; retry < 3; retry++ {
			if err := images.PushToZarfRegistry(tempPath.images, component.Images); err != nil {
				message.Errorf(err, "Unable to push images to the Zarf Registry, retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			} else {
				break
			}
		}

	}

	if hasRepos {
		// Try repo push up to 3 times
		for retry := 0; retry < 3; retry++ {
			// Push all the repos from the extracted archive
			if err := git.PushAllDirectories(componentPath.repos); err != nil {
				message.Errorf(err, "Unable to push repos to the Zarf Registry, retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
				continue
			} else {
				break
			}
		}
	}

	// Start any data injection async
	if hasDataInjections {
		var waitGroup sync.WaitGroup

		message.Info("Loading data injections")
		for _, data := range component.DataInjections {
			waitGroup.Add(1)
			go handleDataInjection(&waitGroup, data, componentPath)
		}
		defer waitGroup.Wait()
	}

	if len(component.Charts) > 0 || len(component.Manifests) > 0 {
		for _, chart := range component.Charts {
			// zarf magic for the value file
			for idx := range chart.ValuesFiles {
				chartValueName := helm.StandardName(componentPath.values, chart) + "-" + strconv.Itoa(idx)
				valueTemplate.Apply(component, chartValueName)
			}

			// Generate helm templates to pass to gitops engine
			addedConnectStrings, installedChartName := helm.InstallOrUpgradeChart(helm.ChartOptions{
				BasePath:  componentPath.base,
				Chart:     chart,
				Component: component,
			})
			installedCharts = append(installedCharts, types.InstalledCharts{Namespace: chart.Namespace, ChartName: installedChartName})

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
			addedConnectStrings, installedChartName := helm.GenerateChart(componentPath.manifests, manifest, component)
			installedCharts = append(installedCharts, types.InstalledCharts{Namespace: manifest.DefaultNamespace, ChartName: installedChartName})
			for name, description := range addedConnectStrings {
				connectStrings[name] = description
			}
		}
	}

	for _, script := range component.Scripts.After {
		loopScriptUntilSuccess(script, component.Scripts)
	}

	if isSeedRegistry {
		postSeedRegistry(tempPath)
	}

	return installedCharts
}

func packageUsesK8s(zarfPackage types.ZarfPackage) bool {
	for _, component := range zarfPackage.Components {
		// If the component is using anything that depends on the cluster, return true
		if len(component.Charts) > 0 ||
			len(component.Images) > 0 ||
			len(component.Repos) > 0 ||
			len(component.Manifests) > 0 {
			return true
		}
	}
	return false
}
