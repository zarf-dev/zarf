// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
)

// Deploy attempts to deploy the given PackageConfig.
func (p *Packager) Deploy() (err error) {
	if err = p.source.LoadPackage(p.layout, true); err != nil {
		return fmt.Errorf("unable to load the package: %w", err)
	}

	if err = p.readZarfYAML(p.layout.ZarfYAML); err != nil {
		return err
	}

	if err := p.validateLastNonBreakingVersion(); err != nil {
		return err
	}

	if err := p.stageSBOMViewFiles(); err != nil {
		return err
	}

	// Confirm the overall package deployment
	if !p.confirmAction(config.ZarfDeployStage) {
		return fmt.Errorf("deployment cancelled")
	}

	// Set variables and prompt if --confirm is not set
	if err := p.setVariableMapInConfig(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	p.hpaModified = false
	p.connectStrings = make(types.ConnectStrings)
	// Reset registry HPA scale down whether an error occurs or not
	defer func() {
		if p.isConnectedToCluster() && p.hpaModified {
			if err := p.cluster.EnableRegHPAScaleDown(); err != nil {
				message.Debugf("unable to reenable the registry HPA scale down: %s", err.Error())
			}
		}
	}()

	// Filter out components that are not compatible with this system
	p.filterComponents()

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents()
	if err != nil {
		return err
	}
	if len(deployedComponents) == 0 {
		message.Warn("No components were selected for deployment.  Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf deployment complete")

	p.printTablesForDeployment(deployedComponents)

	return nil
}

// deployComponents loops through a list of ZarfComponents and deploys them.
func (p *Packager) deployComponents() (deployedComponents []types.DeployedComponent, err error) {
	componentsToDeploy := p.getValidComponents()

	// Generate a value template
	if p.valueTemplate, err = template.Generate(p.cfg); err != nil {
		return deployedComponents, fmt.Errorf("unable to generate the value template: %w", err)
	}

	// Check if this package has been deployed before and grab relevant information about already deployed components
	if p.generation == 0 {
		p.generation = 1 // If this is the first deployment, set the generation to 1
	}

	// Process all the components we are deploying
	for _, component := range componentsToDeploy {

		deployedComponent := types.DeployedComponent{
			Name:               component.Name,
			Status:             types.ComponentStatusDeploying,
			ObservedGeneration: p.generation,
		}

		// If this component requires a cluster, connect to one
		if p.requiresCluster(component) {
			timeout := cluster.DefaultTimeout
			if p.isInitConfig() {
				timeout = 5 * time.Minute
			}

			if err := p.connectToCluster(timeout); err != nil {
				return deployedComponents, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
			}
		}

		// Ensure we don't overwrite any installedCharts data when updating the package secret
		if p.isConnectedToCluster() {
			deployedComponent.InstalledCharts, err = p.cluster.GetInstalledChartsForComponent(p.cfg.Pkg.Metadata.Name, component)
			if err != nil {
				message.Debugf("Unable to fetch installed Helm charts for component '%s': %s", component.Name, err.Error())
			}
		}

		deployedComponents = append(deployedComponents, deployedComponent)
		idx := len(deployedComponents) - 1

		// Update the package secret to indicate that we are attempting to deploy this component
		if p.isConnectedToCluster() {
			if _, err := p.cluster.RecordPackageDeploymentAndWait(p.cfg.Pkg, deployedComponents, p.connectStrings, p.generation, component, p.cfg.DeployOpts.SkipWebhooks); err != nil {
				message.Debugf("Unable to record package deployment for component %s: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
			}
		}

		// Deploy the component
		var charts []types.InstalledChart
		var deployErr error
		if p.isInitConfig() {
			charts, deployErr = p.deployInitComponent(component)
		} else {
			charts, deployErr = p.deployComponent(component, false /* keep img checksum */, false /* always push images */)
		}

		onDeploy := component.Actions.OnDeploy

		onFailure := func() {
			if err := p.runActions(onDeploy.Defaults, onDeploy.OnFailure, p.valueTemplate); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}

		if deployErr != nil {
			onFailure()

			// Update the package secret to indicate that we failed to deploy this component
			deployedComponents[idx].Status = types.ComponentStatusFailed
			if p.isConnectedToCluster() {
				if _, err := p.cluster.RecordPackageDeploymentAndWait(p.cfg.Pkg, deployedComponents, p.connectStrings, p.generation, component, p.cfg.DeployOpts.SkipWebhooks); err != nil {
					message.Debugf("Unable to record package deployment for component %q: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
				}
			}

			return deployedComponents, fmt.Errorf("unable to deploy component %q: %w", component.Name, deployErr)
		}

		// Update the package secret to indicate that we successfully deployed this component
		deployedComponents[idx].InstalledCharts = charts
		deployedComponents[idx].Status = types.ComponentStatusSucceeded
		if p.isConnectedToCluster() {
			if _, err := p.cluster.RecordPackageDeploymentAndWait(p.cfg.Pkg, deployedComponents, p.connectStrings, p.generation, component, p.cfg.DeployOpts.SkipWebhooks); err != nil {
				message.Debugf("Unable to record package deployment for component %q: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
			}
		}

		if err := p.runActions(onDeploy.Defaults, onDeploy.OnSuccess, p.valueTemplate); err != nil {
			onFailure()
			return deployedComponents, fmt.Errorf("unable to run component success action: %w", err)
		}
	}

	return deployedComponents, nil
}

func (p *Packager) deployInitComponent(component types.ZarfComponent) (charts []types.InstalledChart, err error) {
	hasExternalRegistry := p.cfg.InitOpts.RegistryInfo.Address != ""
	isSeedRegistry := component.Name == "zarf-seed-registry"
	isRegistry := component.Name == "zarf-registry"
	isInjector := component.Name == "zarf-injector"
	isAgent := component.Name == "zarf-agent"

	// Always init the state before the first component that requires the cluster (on most deployments, the zarf-seed-registry)
	if p.requiresCluster(component) && p.cfg.State == nil {
		err = p.cluster.InitZarfState(p.cfg.InitOpts)
		if err != nil {
			return charts, fmt.Errorf("unable to initialize Zarf state: %w", err)
		}
	}

	if hasExternalRegistry && (isSeedRegistry || isInjector || isRegistry) {
		message.Notef("Not deploying the component (%s) since external registry information was provided during `zarf init`", component.Name)
		return charts, nil
	}

	if isRegistry {
		// If we are deploying the registry then mark the HPA as "modifed" to set it to Min later
		p.hpaModified = true
	}

	// Before deploying the seed registry, start the injector
	if isSeedRegistry {
		p.cluster.StartInjectionMadness(p.layout.Base, p.layout.Images.Base, component.Images)
	}

	charts, err = p.deployComponent(component, isAgent /* skip img checksum if isAgent */, isSeedRegistry /* skip image push if isSeedRegistry */)
	if err != nil {
		return charts, fmt.Errorf("unable to deploy component %q: %w", component.Name, err)
	}

	// Do cleanup for when we inject the seed registry during initialization
	if isSeedRegistry {
		if err := p.cluster.StopInjectionMadness(); err != nil {
			return charts, fmt.Errorf("unable to seed the Zarf Registry: %w", err)
		}
	}

	return charts, nil
}

// Deploy a Zarf Component.
func (p *Packager) deployComponent(component types.ZarfComponent, noImgChecksum bool, noImgPush bool) (charts []types.InstalledChart, err error) {
	// Toggles for general deploy operations
	componentPath := p.layout.Components.Dirs[component.Name]

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	hasImages := len(component.Images) > 0 && !noImgPush
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasDataInjections := len(component.DataInjections) > 0
	hasFiles := len(component.Files) > 0

	onDeploy := component.Actions.OnDeploy

	if err = p.runActions(onDeploy.Defaults, onDeploy.Before, p.valueTemplate); err != nil {
		return charts, fmt.Errorf("unable to run component before action: %w", err)
	}

	if hasFiles {
		if err := p.processComponentFiles(component, componentPath.Files); err != nil {
			return charts, fmt.Errorf("unable to process the component files: %w", err)
		}
	}

	if !p.valueTemplate.Ready() && p.requiresCluster(component) {
		// Setup the state in the config and get the valuesTemplate
		p.valueTemplate, err = p.setupStateValuesTemplate()
		if err != nil {
			return charts, err
		}

		// Disable the registry HPA scale down if we are deploying images and it is not already disabled
		if hasImages && !p.hpaModified && p.cfg.State.RegistryInfo.InternalRegistry {
			if err := p.cluster.DisableRegHPAScaleDown(); err != nil {
				message.Debugf("unable to disable the registry HPA scale down: %s", err.Error())
			} else {
				p.hpaModified = true
			}
		}
	}

	if hasImages {
		if err := p.pushImagesToRegistry(component.Images, noImgChecksum); err != nil {
			return charts, fmt.Errorf("unable to push images to the registry: %w", err)
		}
	}

	if hasRepos {
		if err = p.pushReposToRepository(componentPath.Repos, component.Repos); err != nil {
			return charts, fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	if hasDataInjections {
		waitGroup := sync.WaitGroup{}
		defer waitGroup.Wait()

		for idx, data := range component.DataInjections {
			waitGroup.Add(1)
			go p.cluster.HandleDataInjection(&waitGroup, data, componentPath, idx)
		}
	}

	if hasCharts || hasManifests {
		if charts, err = p.installChartAndManifests(componentPath, component); err != nil {
			return charts, fmt.Errorf("unable to install helm chart(s): %w", err)
		}
	}

	if err = p.runActions(onDeploy.Defaults, onDeploy.After, p.valueTemplate); err != nil {
		return charts, fmt.Errorf("unable to run component after action: %w", err)
	}

	return charts, nil
}

// Move files onto the host of the machine performing the deployment.
func (p *Packager) processComponentFiles(component types.ZarfComponent, pkgLocation string) error {
	spinner := message.NewProgressSpinner("Copying %d files", len(component.Files))
	defer spinner.Stop()

	for fileIdx, file := range component.Files {
		spinner.Updatef("Loading %s", file.Target)

		fileLocation := filepath.Join(pkgLocation, strconv.Itoa(fileIdx), filepath.Base(file.Target))
		if utils.InvalidPath(fileLocation) {
			fileLocation = filepath.Join(pkgLocation, strconv.Itoa(fileIdx))
		}

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			spinner.Updatef("Validating SHASUM for %s", file.Target)
			if err := utils.SHAsMatch(fileLocation, file.Shasum); err != nil {
				return err
			}
		}

		// Replace temp target directory and home directory
		file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", p.layout.Base, 1)
		file.Target = config.GetAbsHomePath(file.Target)

		fileList := []string{}
		if utils.IsDir(fileLocation) {
			files, _ := utils.RecursiveFileList(fileLocation, nil, false)
			fileList = append(fileList, files...)
		} else {
			fileList = append(fileList, fileLocation)
		}

		for _, subFile := range fileList {
			// Check if the file looks like a text file
			isText, err := utils.IsTextFile(subFile)
			if err != nil {
				message.Debugf("unable to determine if file %s is a text file: %s", subFile, err)
			}

			// If the file is a text file, template it
			if isText {
				spinner.Updatef("Templating %s", file.Target)
				if err := p.valueTemplate.Apply(component, subFile, true); err != nil {
					return fmt.Errorf("unable to template file %s: %w", subFile, err)
				}
			}
		}

		// Copy the file to the destination
		spinner.Updatef("Saving %s", file.Target)
		err := utils.CreatePathAndCopy(fileLocation, file.Target)
		if err != nil {
			return fmt.Errorf("unable to copy file %s to %s: %w", fileLocation, file.Target, err)
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
				return fmt.Errorf("unable to create symlink %s->%s: %w", link, file.Target, err)
			}
		}

		// Cleanup now to reduce disk pressure
		_ = os.RemoveAll(fileLocation)
	}

	spinner.Success()

	return nil
}

// setupStateValuesTemplate fetched the current ZarfState from the k8s cluster and generate a p.valueTemplate from the state values.
func (p *Packager) setupStateValuesTemplate() (values *template.Values, err error) {
	// If we are touching K8s, make sure we can talk to it once per deployment
	spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
	defer spinner.Stop()

	state, err := p.cluster.LoadZarfState()
	// Return on error if we are not in YOLO mode
	if err != nil && !p.cfg.Pkg.Metadata.YOLO {
		return nil, fmt.Errorf("%s %w", lang.ErrLoadState, err)
	} else if state == nil && p.cfg.Pkg.Metadata.YOLO {
		state = &types.ZarfState{}
		// YOLO mode, so minimal state needed
		state.Distro = "YOLO"

		// Try to create the zarf namespace
		spinner.Updatef("Creating the Zarf namespace")
		zarfNamespace := p.cluster.NewZarfManagedNamespace(cluster.ZarfNamespaceName)
		if _, err := p.cluster.CreateNamespace(zarfNamespace); err != nil {
			spinner.Fatalf(err, "Unable to create the zarf namespace")
		}
	}

	if p.cfg.Pkg.Metadata.YOLO && state.Distro != "YOLO" {
		message.Warn("This package is in YOLO mode, but the cluster was already initialized with 'zarf init'. " +
			"This may cause issues if the package does not exclude any charts or manifests from the Zarf Agent using " +
			"the pod or namespace label `zarf.dev/agent: ignore'.")
	}

	p.cfg.State = state

	// Continue loading state data if it is valid
	values, err = template.Generate(p.cfg)
	if err != nil {
		return values, err
	}

	spinner.Success()
	return values, nil
}

// Push all of the components images to the configured container registry.
func (p *Packager) pushImagesToRegistry(componentImages []string, noImgChecksum bool) error {
	if len(componentImages) == 0 {
		return nil
	}

	var combinedImageList []transform.Image
	for _, src := range componentImages {
		ref, err := transform.ParseImageRef(src)
		if err != nil {
			return fmt.Errorf("failed to create ref for image %s: %w", src, err)
		}
		combinedImageList = append(combinedImageList, ref)
	}

	imageList := helpers.Unique(combinedImageList)

	imgConfig := images.ImageConfig{
		ImagesPath:    p.layout.Images.Base,
		ImageList:     imageList,
		NoChecksum:    noImgChecksum,
		RegInfo:       p.cfg.State.RegistryInfo,
		Insecure:      config.CommonOptions.Insecure,
		Architectures: []string{p.cfg.Pkg.Metadata.Architecture, p.cfg.Pkg.Build.Architecture},
	}

	return helpers.Retry(func() error {
		return imgConfig.PushToZarfRegistry()
	}, 3, 5*time.Second, message.Warnf)
}

// Push all of the components git repos to the configured git server.
func (p *Packager) pushReposToRepository(reposPath string, repos []string) error {
	for _, repoURL := range repos {
		// Create an anonymous function to push the repo to the Zarf git server
		tryPush := func() error {
			gitClient := git.New(p.cfg.State.GitServer)
			svcInfo, _ := k8s.ServiceInfoFromServiceURL(gitClient.Server.Address)

			// If this is a service (svcInfo is not nil), create a port-forward tunnel to that resource
			if svcInfo != nil {
				if !p.isConnectedToCluster() {
					err := p.connectToCluster(5 * time.Second)
					if err != nil {
						return err
					}
				}

				tunnel, err := p.cluster.NewTunnel(svcInfo.Namespace, k8s.SvcResource, svcInfo.Name, "", 0, svcInfo.Port)
				if err != nil {
					return err
				}

				_, err = tunnel.Connect()
				if err != nil {
					return err
				}
				defer tunnel.Close()
				gitClient.Server.Address = tunnel.HTTPEndpoint()
			}

			return gitClient.PushRepo(repoURL, reposPath)
		}

		// Try repo push up to 3 times
		if err := helpers.Retry(tryPush, 3, 5*time.Second, message.Warnf); err != nil {
			return fmt.Errorf("unable to push repo %s to the Git Server: %w", repoURL, err)
		}
	}

	return nil
}

// Install all Helm charts and raw k8s manifests into the k8s cluster.
func (p *Packager) installChartAndManifests(componentPaths *layout.ComponentPaths, component types.ZarfComponent) (installedCharts []types.InstalledChart, err error) {
	for _, chart := range component.Charts {

		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			chartValueName := fmt.Sprintf("%s-%d", helm.StandardName(componentPaths.Values, chart), idx)
			if err := p.valueTemplate.Apply(component, chartValueName, false); err != nil {
				return installedCharts, err
			}
		}

		// TODO (@WSTARR): Currently this logic is library-only and is untested while it is in an experimental state - it may eventually get added as shorthand in Zarf Variables though
		var valuesOverrides map[string]any
		if componentChartValuesOverrides, ok := p.cfg.DeployOpts.ValuesOverridesMap[component.Name]; ok {
			if chartValuesOverrides, ok := componentChartValuesOverrides[chart.Name]; ok {
				valuesOverrides = chartValuesOverrides
			}
		}

		helmCfg := helm.New(
			chart,
			componentPaths.Charts,
			componentPaths.Values,
			helm.WithDeployInfo(
				component,
				p.cfg,
				p.cluster,
				valuesOverrides,
				p.cfg.DeployOpts.Timeout),
		)

		addedConnectStrings, installedChartName, err := helmCfg.InstallOrUpgradeChart()
		if err != nil {
			return installedCharts, err
		}
		installedCharts = append(installedCharts, types.InstalledChart{Namespace: chart.Namespace, ChartName: installedChartName})

		// Iterate over any connectStrings and add to the main map
		for name, description := range addedConnectStrings {
			p.connectStrings[name] = description
		}
	}

	for _, manifest := range component.Manifests {
		for idx := range manifest.Files {
			if utils.InvalidPath(filepath.Join(componentPaths.Manifests, manifest.Files[idx])) {
				// The path is likely invalid because of how we compose OCI components, add an index suffix to the filename
				manifest.Files[idx] = fmt.Sprintf("%s-%d.yaml", manifest.Name, idx)
				if utils.InvalidPath(filepath.Join(componentPaths.Manifests, manifest.Files[idx])) {
					return installedCharts, fmt.Errorf("unable to find manifest file %s", manifest.Files[idx])
				}
			}
		}
		// Move kustomizations to files now
		for idx := range manifest.Kustomizations {
			kustomization := fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, idx)
			manifest.Files = append(manifest.Files, kustomization)
		}

		if manifest.Namespace == "" {
			// Helm gets sad when you don't provide a namespace even though we aren't using helm templating
			manifest.Namespace = corev1.NamespaceDefault
		}

		// Create a chart and helm cfg from a given Zarf Manifest.
		helmCfg, err := helm.NewFromZarfManifest(
			manifest,
			componentPaths.Manifests,
			p.cfg.Pkg.Metadata.Name,
			component.Name,
			helm.WithDeployInfo(
				component,
				p.cfg,
				p.cluster,
				nil,
				p.cfg.DeployOpts.Timeout),
		)
		if err != nil {
			return installedCharts, err
		}

		// Install the chart.
		addedConnectStrings, installedChartName, err := helmCfg.InstallOrUpgradeChart()
		if err != nil {
			return installedCharts, err
		}

		installedCharts = append(installedCharts, types.InstalledChart{Namespace: manifest.Namespace, ChartName: installedChartName})

		// Iterate over any connectStrings and add to the main map
		for name, description := range addedConnectStrings {
			p.connectStrings[name] = description
		}
	}

	return installedCharts, nil
}

func (p *Packager) printTablesForDeployment(componentsToDeploy []types.DeployedComponent) {

	// If not init config, print the application connection table
	if !p.isInitConfig() {
		message.PrintConnectStringTable(p.connectStrings)
	} else {
		// Grab a fresh copy of the state (if we are able) to print the most up-to-date version of the creds
		freshState, err := p.cluster.LoadZarfState()
		if err != nil {
			freshState = p.cfg.State
		}
		// otherwise, print the init config connection and passwords
		message.PrintCredentialTable(freshState, componentsToDeploy)
	}
}
