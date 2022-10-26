package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/git"
	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/images"
	"github.com/defenseunicorns/zarf/src/internal/sbom"
	"github.com/defenseunicorns/zarf/src/internal/template"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
)

var valueTemplate template.Values
var connectStrings = make(types.ConnectStrings)

// Deploy attempts to deploy a Zarf package that is define within the global DeployOptions struct
func (p *Package) Deploy() error {
	message.Debug("packager.Deploy()")

	spinner := message.NewProgressSpinner("Preparing to deploy Zarf Package %s", p.cfg.DeployOpts.PackagePath)
	defer spinner.Stop()

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(p.cfg.DeployOpts.PackagePath) {
		return fmt.Errorf("unable to find the package at %s", p.cfg.DeployOpts.PackagePath)
	}

	// Extract the archive
	spinner.Updatef("Extracting the package, this may take a few moments")
	if err := archiver.Unarchive(p.cfg.DeployOpts.PackagePath, p.tmp.Base); err != nil {
		return fmt.Errorf("unable to extract the package: %w", err)
	}

	// Load the config from the extracted archive zarf.yaml
	spinner.Updatef("Loading the zarf package config")
	configPath := filepath.Join(p.tmp.Base, "zarf.yaml")
	if err := config.LoadConfig(configPath, true); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
	}

	// If init config, make sure things are ready
	if p.cfg.IsInitConfig {
		utils.RunPreflightChecks()
	}

	spinner.Success()

	// If SBOM files exist, temporary place them in the deploy directory
	sbomViewFiles, _ := filepath.Glob(filepath.Join(p.tmp.Sboms, "sbom-viewer-*"))
	if err := sbom.WriteSBOMFiles(sbomViewFiles); err != nil {
		// Don't stop the deployment, let the user decide if they want to continue the deployment
		message.Errorf(err, "Unable to process the SBOM files for this package")
	}

	// Confirm the overall package deployment
	confirm := confirmAction("Deploy", sbomViewFiles)

	// Don't continue unless the user says so
	if !confirm {
		return fmt.Errorf("user cancelled the deployment")
	}

	// Set variables and prompt if --confirm is not set
	if err := config.SetActiveVariables(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents()
	if err != nil {
		return fmt.Errorf("unable to deploy all components in this Zarf Package: %w", err)
	}

	// Notify all the things about the successful deployment
	message.SuccessF("Zarf deployment complete")
	p.printTablesForDeployment(deployedComponents)

	// Save deployed package information to k8s
	// Note: Not all packages need k8s; check if k8s is being used before saving the secret
	if p.cluster != nil {
		p.cluster.RecordPackageDeployment(p.cfg.pkg, deployedComponents)
	}

	return nil
}

// deployComponents loops through a list of ZarfComponents and deploys them
func (p *Package) deployComponents() (deployedComponents []types.DeployedComponent, err error) {
	componentsToDeploy := p.getValidComponents()
	config.SetDeployingComponents(deployedComponents)

	for _, component := range componentsToDeploy {
		var installedCharts []types.InstalledChart

		deployedComponent := types.DeployedComponent{Name: component.Name}
		addChecksumToImg := true

		if p.cfg.IsInitConfig {
			installedCharts, err = p.deployInitComponent(component)
			if err != nil {
				return deployedComponents, fmt.Errorf("unable to deploy component %s: %w", component.Name, err)
			}
		} else {
			installedCharts = p.deployComponent(component, addChecksumToImg)
		}

		// Deploy the component
		deployedComponent.InstalledCharts = installedCharts
		deployedComponents = append(deployedComponents, deployedComponent)
		config.SetDeployingComponents(deployedComponents)
	}

	config.ClearDeployingComponents()
	return deployedComponents, nil
}

func (p *Package) deployInitComponent(component types.ZarfComponent) (installedCharts []types.InstalledChart, err error) {
	hasExternalRegistry := p.cfg.InitOpts.RegistryInfo.Address != ""
	isSeedRegistry := component.Name == "seed-registry"
	isRegistry := component.Name == "zarf-registry"
	isInjector := component.Name == "zarf-injector"
	isAgent := component.Name == "zarf-agent"

	// Always init the state on the seed registry component
	if isSeedRegistry {
		p.cluster, err = cluster.NewClusterWithWait(5 * time.Minute)
		if err != nil {
			return installedCharts, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
		}
		p.cluster.InitZarfState(p.tmp, p.cfg.InitOpts)
	}

	if hasExternalRegistry && (isSeedRegistry || isInjector || isRegistry) {
		message.Notef("Not deploying the component (%s) since external registry information was provided during `zarf init`", component.Name)
		return installedCharts, nil
	}

	// Before deploying the seed registry, start the injector
	if isSeedRegistry {
		p.cluster.RunInjectionMadness(p.tmp)
	}

	installedCharts = p.deployComponent(component, !isAgent)

	// Do cleanup for when we inject the seed registry during initialization
	if isSeedRegistry {
		err := p.cluster.PostSeedRegistry(p.tmp)
		if err != nil {
			return installedCharts, fmt.Errorf("unable to seed the Zarf Registry: %w", err)
		}

		// Push the seed images into to Zarf registry
		err = images.PushToZarfRegistry(p.tmp.SeedImage, []string{config.ZarfSeedImage}, false)
		if err != nil {
			return installedCharts, fmt.Errorf("unable to push the seed images to the Zarf Registry: %w", err)
		}
	}

	return installedCharts, nil
}

// Deploy a Zarf Component
func (p *Package) deployComponent(component types.ZarfComponent, addChecksumToImgs bool) []types.InstalledChart {
	var installedCharts []types.InstalledChart
	message.Debugf("packager.deployComponent(%#v, %#v", p.tmp, component)

	// Toggles for general deploy operations
	componentPath, err := p.createComponentPaths(component)
	if err != nil {
		message.Fatalf(err, "Unable to create the component paths")
	}

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	hasImages := len(component.Images) > 0
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasDataInjections := len(component.DataInjections) > 0

	// Run the 'before' scripts and move files before we do anything else
	p.runComponentScripts(component.Scripts.Before, component.Scripts)
	p.processComponentFiles(component.Files, componentPath.Files)

	// Generate a value template
	valueTemplate = template.Generate()
	if !valueTemplate.Ready() && (hasImages || hasCharts || hasManifests || hasRepos) {
		valueTemplate = p.getUpdatedValueTemplate(component)
	}

	/* Install all the parts of the component */
	if hasImages {
		p.pushImagesToRegistry(component.Images, addChecksumToImgs)
	}

	if hasRepos {
		p.pushReposToRepository(componentPath.Repos, component.Repos)
	}

	if hasDataInjections {
		waitGroup := sync.WaitGroup{}
		defer waitGroup.Wait()
		p.performDataInjections(&waitGroup, componentPath, component.DataInjections)
	}

	if hasCharts || hasManifests {
		installedCharts = p.installChartAndManifests(componentPath, component)
	}

	// Run the 'after' scripts after all other attributes of the component has been deployed
	p.runComponentScripts(component.Scripts.After, component.Scripts)

	return installedCharts
}

// Move files onto the host of the machine performing the deployment
func (p *Package) processComponentFiles(componentFiles []types.ZarfFile, sourceLocation string) {
	var spinner message.Spinner
	if len(componentFiles) > 0 {
		spinner = *message.NewProgressSpinner("Copying %d files", len(componentFiles))
		defer spinner.Stop()
	}

	for index, file := range componentFiles {
		spinner.Updatef("Loading %s", file.Target)
		sourceFile := filepath.Join(sourceLocation, strconv.Itoa(index))

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			spinner.Updatef("Validating SHASUM for %s", file.Target)
			utils.ValidateSha256Sum(file.Shasum, sourceFile)
		}

		// Replace temp target directories
		file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", p.tmp.Base, 1)

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
func (p *Package) getUpdatedValueTemplate(component types.ZarfComponent) template.Values {
	// If we are touching K8s, make sure we can talk to it once per deployment
	spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
	defer spinner.Stop()

	state, err := p.cluster.LoadZarfState()
	if err != nil {
		spinner.Fatalf(err, "Unable to load the Zarf State from the Kubernetes cluster")
	}

	if state.Distro == "" {
		// If no distro the zarf secret did not load properly
		spinner.Fatalf(nil, "Unable to load the zarf/zarf-state secret, did you remember to run zarf init first?")
	}

	// Continue loading state data if it is valid
	config.InitState(state)
	valueTemplate := template.Generate()
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
func (p *Package) pushImagesToRegistry(componentImages []string, addShasumToImg bool) {
	if len(componentImages) == 0 {
		return
	}

	// Try image push up to 3 times
	for retry := 0; retry < 3; retry++ {
		if err := images.PushToZarfRegistry(p.tmp.Images, componentImages, addShasumToImg); err != nil {
			message.Errorf(err, "Unable to push images to the Registry, retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		} else {
			break
		}
	}
}

// Push all of the components git repos to the configured git server
func (p *Package) pushReposToRepository(reposPath string, repos []string) {
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
func (p *Package) performDataInjections(waitGroup *sync.WaitGroup, componentPath types.ComponentPaths, dataInjections []types.ZarfDataInjection) {
	if len(dataInjections) > 0 {
		message.Info("Loading data injections")
	}

	for _, data := range dataInjections {
		waitGroup.Add(1)
		go p.cluster.HandleDataInjection(waitGroup, data, componentPath)
	}
}

// Install all Helm charts and raw k8s manifests into the k8s cluster
func (p *Package) installChartAndManifests(componentPath types.ComponentPaths, component types.ZarfComponent) []types.InstalledChart {
	installedCharts := []types.InstalledChart{}

	for _, chart := range component.Charts {
		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			chartValueName := helm.StandardName(componentPath.Values, chart) + "-" + strconv.Itoa(idx)
			valueTemplate.Apply(component, chartValueName)
		}

		// Generate helm templates to pass to gitops engine
		addedConnectStrings, installedChartName := helm.InstallOrUpgradeChart(helm.ChartOptions{
			BasePath:  componentPath.Base,
			Chart:     chart,
			Component: component,
		})
		installedCharts = append(installedCharts, types.InstalledChart{Namespace: chart.Namespace, ChartName: installedChartName})

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

		if manifest.Namespace == "" {
			// Helm gets sad when you don't provide a namespace even though we aren't using helm templating
			manifest.Namespace = corev1.NamespaceDefault
		}

		// Iterate over any connectStrings and add to the main map
		addedConnectStrings, installedChartName := helm.GenerateChart(componentPath.Manifests, manifest, component)
		installedCharts = append(installedCharts, types.InstalledChart{Namespace: manifest.Namespace, ChartName: installedChartName})

		// Iterate over any connectStrings and add to the main map
		for name, description := range addedConnectStrings {
			connectStrings[name] = description
		}
	}

	return installedCharts
}

func (p *Package) printTablesForDeployment(componentsToDeploy []types.DeployedComponent) {
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
}
