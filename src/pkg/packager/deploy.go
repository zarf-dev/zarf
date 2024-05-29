// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/defenseunicorns/pkg/helpers"
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
	"github.com/defenseunicorns/zarf/src/pkg/packager/actions"
	"github.com/defenseunicorns/zarf/src/pkg/packager/filters"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
)

func (p *Packager) resetRegistryHPA(ctx context.Context) {
	if p.isConnectedToCluster() && p.hpaModified {
		if err := p.cluster.EnableRegHPAScaleDown(ctx); err != nil {
			message.Debugf("unable to reenable the registry HPA scale down: %s", err.Error())
		}
	}
}

// Deploy attempts to deploy the given PackageConfig.
func (p *Packager) Deploy(ctx context.Context) (err error) {

	isInteractive := !config.CommonOptions.Confirm

	deployFilter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ForDeploy(p.cfg.PkgOpts.OptionalComponents, isInteractive),
	)

	if isInteractive {
		filter := filters.Empty()

		p.cfg.Pkg, p.warnings, err = p.source.LoadPackage(p.layout, filter, true)
		if err != nil {
			return fmt.Errorf("unable to load the package: %w", err)
		}
	} else {
		p.cfg.Pkg, p.warnings, err = p.source.LoadPackage(p.layout, deployFilter, true)
		if err != nil {
			return fmt.Errorf("unable to load the package: %w", err)
		}

		if err := p.populatePackageVariableConfig(); err != nil {
			return fmt.Errorf("unable to set the active variables: %w", err)
		}
	}

	if err := p.validateLastNonBreakingVersion(); err != nil {
		return err
	}

	var sbomWarnings []string
	p.sbomViewFiles, sbomWarnings, err = p.layout.SBOMs.StageSBOMViewFiles()
	if err != nil {
		return err
	}

	p.warnings = append(p.warnings, sbomWarnings...)

	// Confirm the overall package deployment
	if !p.confirmAction(config.ZarfDeployStage) {
		return fmt.Errorf("deployment cancelled")
	}

	if isInteractive {
		p.cfg.Pkg.Components, err = deployFilter.Apply(p.cfg.Pkg)
		if err != nil {
			return err
		}

		// Set variables and prompt if --confirm is not set
		if err := p.populatePackageVariableConfig(); err != nil {
			return fmt.Errorf("unable to set the active variables: %w", err)
		}
	}

	p.hpaModified = false
	p.connectStrings = make(types.ConnectStrings)
	// Reset registry HPA scale down whether an error occurs or not
	defer p.resetRegistryHPA(ctx)

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents(ctx)
	if err != nil {
		return err
	}
	if len(deployedComponents) == 0 {
		message.Warn("No components were selected for deployment.  Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf deployment complete")

	p.printTablesForDeployment(ctx, deployedComponents)

	return nil
}

// deployComponents loops through a list of ZarfComponents and deploys them.
func (p *Packager) deployComponents(ctx context.Context) (deployedComponents []types.DeployedComponent, err error) {
	// Check if this package has been deployed before and grab relevant information about already deployed components
	if p.generation == 0 {
		p.generation = 1 // If this is the first deployment, set the generation to 1
	}

	// Process all the components we are deploying
	for _, component := range p.cfg.Pkg.Components {

		deployedComponent := types.DeployedComponent{
			Name:               component.Name,
			Status:             types.ComponentStatusDeploying,
			ObservedGeneration: p.generation,
		}

		// If this component requires a cluster, connect to one
		if component.RequiresCluster() {
			timeout := cluster.DefaultTimeout
			if p.cfg.Pkg.IsInitConfig() {
				timeout = 5 * time.Minute
			}
			connectCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if err := p.connectToCluster(connectCtx); err != nil {
				return deployedComponents, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
			}
		}

		// Ensure we don't overwrite any installedCharts data when updating the package secret
		if p.isConnectedToCluster() {
			deployedComponent.InstalledCharts, err = p.cluster.GetInstalledChartsForComponent(ctx, p.cfg.Pkg.Metadata.Name, component)
			if err != nil {
				message.Debugf("Unable to fetch installed Helm charts for component '%s': %s", component.Name, err.Error())
			}
		}

		deployedComponents = append(deployedComponents, deployedComponent)
		idx := len(deployedComponents) - 1

		// Update the package secret to indicate that we are attempting to deploy this component
		if p.isConnectedToCluster() {
			if _, err := p.cluster.RecordPackageDeploymentAndWait(ctx, p.cfg.Pkg, deployedComponents, p.connectStrings, p.generation, component, p.cfg.DeployOpts.SkipWebhooks); err != nil {
				message.Debugf("Unable to record package deployment for component %s: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
			}
		}

		// Deploy the component
		var charts []types.InstalledChart
		var deployErr error
		if p.cfg.Pkg.IsInitConfig() {
			charts, deployErr = p.deployInitComponent(ctx, component)
		} else {
			charts, deployErr = p.deployComponent(ctx, component, false /* keep img checksum */, false /* always push images */)
		}

		onDeploy := component.Actions.OnDeploy

		onFailure := func() {
			if err := actions.Run(onDeploy.Defaults, onDeploy.OnFailure, p.variableConfig); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}

		if deployErr != nil {
			onFailure()

			// Update the package secret to indicate that we failed to deploy this component
			deployedComponents[idx].Status = types.ComponentStatusFailed
			if p.isConnectedToCluster() {
				if _, err := p.cluster.RecordPackageDeploymentAndWait(ctx, p.cfg.Pkg, deployedComponents, p.connectStrings, p.generation, component, p.cfg.DeployOpts.SkipWebhooks); err != nil {
					message.Debugf("Unable to record package deployment for component %q: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
				}
			}

			return deployedComponents, fmt.Errorf("unable to deploy component %q: %w", component.Name, deployErr)
		}

		// Update the package secret to indicate that we successfully deployed this component
		deployedComponents[idx].InstalledCharts = charts
		deployedComponents[idx].Status = types.ComponentStatusSucceeded
		if p.isConnectedToCluster() {
			if _, err := p.cluster.RecordPackageDeploymentAndWait(ctx, p.cfg.Pkg, deployedComponents, p.connectStrings, p.generation, component, p.cfg.DeployOpts.SkipWebhooks); err != nil {
				message.Debugf("Unable to record package deployment for component %q: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
			}
		}

		if err := actions.Run(onDeploy.Defaults, onDeploy.OnSuccess, p.variableConfig); err != nil {
			onFailure()
			return deployedComponents, fmt.Errorf("unable to run component success action: %w", err)
		}
	}

	return deployedComponents, nil
}

func (p *Packager) deployInitComponent(ctx context.Context, component types.ZarfComponent) (charts []types.InstalledChart, err error) {
	hasExternalRegistry := p.cfg.InitOpts.RegistryInfo.Address != ""
	isSeedRegistry := component.Name == "zarf-seed-registry"
	isRegistry := component.Name == "zarf-registry"
	isInjector := component.Name == "zarf-injector"
	isAgent := component.Name == "zarf-agent"
	isK3s := component.Name == "k3s"

	if isK3s {
		p.cfg.InitOpts.ApplianceMode = true
	}

	// Always init the state before the first component that requires the cluster (on most deployments, the zarf-seed-registry)
	if component.RequiresCluster() && p.state == nil {
		err = p.cluster.InitZarfState(ctx, p.cfg.InitOpts)
		if err != nil {
			return charts, fmt.Errorf("unable to initialize Zarf state: %w", err)
		}
	}

	if hasExternalRegistry && (isSeedRegistry || isInjector || isRegistry) {
		message.Notef("Not deploying the component (%s) since external registry information was provided during `zarf init`", component.Name)
		return charts, nil
	}

	if isRegistry {
		// If we are deploying the registry then mark the HPA as "modified" to set it to Min later
		p.hpaModified = true
	}

	// Before deploying the seed registry, start the injector
	if isSeedRegistry {
		p.cluster.StartInjectionMadness(ctx, p.layout.Base, p.layout.Images.Base, component.Images)
	}

	charts, err = p.deployComponent(ctx, component, isAgent /* skip img checksum if isAgent */, isSeedRegistry /* skip image push if isSeedRegistry */)
	if err != nil {
		return charts, err
	}

	// Do cleanup for when we inject the seed registry during initialization
	if isSeedRegistry {
		if err := p.cluster.StopInjectionMadness(ctx); err != nil {
			return charts, fmt.Errorf("unable to seed the Zarf Registry: %w", err)
		}
	}

	return charts, nil
}

// Deploy a Zarf Component.
func (p *Packager) deployComponent(ctx context.Context, component types.ZarfComponent, noImgChecksum bool, noImgPush bool) (charts []types.InstalledChart, err error) {
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

	if component.RequiresCluster() {
		// Setup the state in the config
		if p.state == nil {
			err = p.setupState(ctx)
			if err != nil {
				return charts, err
			}
		}

		// Disable the registry HPA scale down if we are deploying images and it is not already disabled
		if hasImages && !p.hpaModified && p.state.RegistryInfo.InternalRegistry {
			if err := p.cluster.DisableRegHPAScaleDown(ctx); err != nil {
				message.Debugf("unable to disable the registry HPA scale down: %s", err.Error())
			} else {
				p.hpaModified = true
			}
		}
	}

	err = p.populateComponentAndStateTemplates(component.Name)
	if err != nil {
		return charts, err
	}

	if err = actions.Run(onDeploy.Defaults, onDeploy.Before, p.variableConfig); err != nil {
		return charts, fmt.Errorf("unable to run component before action: %w", err)
	}

	if hasFiles {
		if err := p.processComponentFiles(component, componentPath.Files); err != nil {
			return charts, fmt.Errorf("unable to process the component files: %w", err)
		}
	}

	if hasImages {
		if err := p.pushImagesToRegistry(ctx, component.Images, noImgChecksum); err != nil {
			return charts, fmt.Errorf("unable to push images to the registry: %w", err)
		}
	}

	if hasRepos {
		if err = p.pushReposToRepository(ctx, componentPath.Repos, component.Repos); err != nil {
			return charts, fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	if hasDataInjections {
		waitGroup := sync.WaitGroup{}
		defer waitGroup.Wait()

		for idx, data := range component.DataInjections {
			waitGroup.Add(1)
			go p.cluster.HandleDataInjection(ctx, &waitGroup, data, componentPath, idx)
		}
	}

	if hasCharts || hasManifests {
		if charts, err = p.installChartAndManifests(componentPath, component); err != nil {
			return charts, err
		}
	}

	if err = actions.Run(onDeploy.Defaults, onDeploy.After, p.variableConfig); err != nil {
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
		if helpers.InvalidPath(fileLocation) {
			fileLocation = filepath.Join(pkgLocation, strconv.Itoa(fileIdx))
		}

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			spinner.Updatef("Validating SHASUM for %s", file.Target)
			if err := helpers.SHAsMatch(fileLocation, file.Shasum); err != nil {
				return err
			}
		}

		// Replace temp target directory and home directory
		file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", p.layout.Base, 1)
		file.Target = config.GetAbsHomePath(file.Target)

		fileList := []string{}
		if helpers.IsDir(fileLocation) {
			files, _ := helpers.RecursiveFileList(fileLocation, nil, false)
			fileList = append(fileList, files...)
		} else {
			fileList = append(fileList, fileLocation)
		}

		for _, subFile := range fileList {
			// Check if the file looks like a text file
			isText, err := helpers.IsTextFile(subFile)
			if err != nil {
				message.Debugf("unable to determine if file %s is a text file: %s", subFile, err)
			}

			// If the file is a text file, template it
			if isText {
				spinner.Updatef("Templating %s", file.Target)
				if err := p.variableConfig.ReplaceTextTemplate(subFile); err != nil {
					return fmt.Errorf("unable to template file %s: %w", subFile, err)
				}
			}
		}

		// Copy the file to the destination
		spinner.Updatef("Saving %s", file.Target)
		err := helpers.CreatePathAndCopy(fileLocation, file.Target)
		if err != nil {
			return fmt.Errorf("unable to copy file %s to %s: %w", fileLocation, file.Target, err)
		}

		// Loop over all symlinks and create them
		for _, link := range file.Symlinks {
			spinner.Updatef("Adding symlink %s->%s", link, file.Target)
			// Try to remove the filepath if it exists
			_ = os.RemoveAll(link)
			// Make sure the parent directory exists
			_ = helpers.CreateParentDirectory(link)
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

// setupState fetches the current ZarfState from the k8s cluster and sets the packager to use it
func (p *Packager) setupState(ctx context.Context) (err error) {
	// If we are touching K8s, make sure we can talk to it once per deployment
	spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
	defer spinner.Stop()

	state, err := p.cluster.LoadZarfState(ctx)
	// Return on error if we are not in YOLO mode
	if err != nil && !p.cfg.Pkg.Metadata.YOLO {
		return fmt.Errorf("%s %w", lang.ErrLoadState, err)
	} else if state == nil && p.cfg.Pkg.Metadata.YOLO {
		state = &types.ZarfState{}
		// YOLO mode, so minimal state needed
		state.Distro = "YOLO"

		// Try to create the zarf namespace
		spinner.Updatef("Creating the Zarf namespace")
		zarfNamespace := cluster.NewZarfManagedNamespace(cluster.ZarfNamespaceName)
		if _, err := p.cluster.CreateNamespace(ctx, zarfNamespace); err != nil {
			spinner.Fatalf(err, "Unable to create the zarf namespace")
		}
	}

	if p.cfg.Pkg.Metadata.YOLO && state.Distro != "YOLO" {
		message.Warn("This package is in YOLO mode, but the cluster was already initialized with 'zarf init'. " +
			"This may cause issues if the package does not exclude any charts or manifests from the Zarf Agent using " +
			"the pod or namespace label `zarf.dev/agent: ignore'.")
	}

	p.state = state

	spinner.Success()
	return nil
}

func (p *Packager) populateComponentAndStateTemplates(componentName string) error {
	applicationTemplates, err := template.GetZarfTemplates(componentName, p.state)
	if err != nil {
		return err
	}
	p.variableConfig.SetApplicationTemplates(applicationTemplates)
	return nil
}

func (p *Packager) populatePackageVariableConfig() error {
	p.variableConfig.SetConstants(p.cfg.Pkg.Constants)
	return p.variableConfig.PopulateVariables(p.cfg.Pkg.Variables, p.cfg.PkgOpts.SetVariables)
}

// Push all of the components images to the configured container registry.
func (p *Packager) pushImagesToRegistry(ctx context.Context, componentImages []string, noImgChecksum bool) error {
	var combinedImageList []transform.Image
	for _, src := range componentImages {
		ref, err := transform.ParseImageRef(src)
		if err != nil {
			return fmt.Errorf("failed to create ref for image %s: %w", src, err)
		}
		combinedImageList = append(combinedImageList, ref)
	}

	imageList := helpers.Unique(combinedImageList)

	pushCfg := images.PushConfig{
		SourceDirectory: p.layout.Images.Base,
		ImageList:       imageList,
		RegInfo:         p.state.RegistryInfo,
		NoChecksum:      noImgChecksum,
		Arch:            p.cfg.Pkg.Build.Architecture,
		Retries:         p.cfg.PkgOpts.Retries,
	}

	return images.Push(ctx, pushCfg)
}

// Push all of the components git repos to the configured git server.
func (p *Packager) pushReposToRepository(ctx context.Context, reposPath string, repos []string) error {
	for _, repoURL := range repos {
		// Create an anonymous function to push the repo to the Zarf git server
		tryPush := func() error {
			gitClient := git.New(p.state.GitServer)
			namespace, name, port, err := serviceInfoFromServiceURL(gitClient.Server.Address)

			// If this is a service (svcInfo is not nil), create a port-forward tunnel to that resource
			// TODO: Find a better way as ignoring the error is not a good solution to decide to port forward.
			if err == nil {
				if !p.isConnectedToCluster() {
					connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					defer cancel()
					err := p.connectToCluster(connectCtx)
					if err != nil {
						return err
					}
				}

				tunnel, err := p.cluster.NewTunnel(namespace, k8s.SvcResource, name, "", 0, port)
				if err != nil {
					return err
				}

				_, err = tunnel.Connect(ctx)
				if err != nil {
					return err
				}
				defer tunnel.Close()
				gitClient.Server.Address = tunnel.HTTPEndpoint()

				return tunnel.Wrap(func() error { return gitClient.PushRepo(repoURL, reposPath) })
			}

			return gitClient.PushRepo(repoURL, reposPath)
		}

		// Try repo push up to retry limit
		if err := helpers.Retry(tryPush, p.cfg.PkgOpts.Retries, 5*time.Second, message.Warnf); err != nil {
			return fmt.Errorf("unable to push repo %s to the Git Server: %w", repoURL, err)
		}
	}

	return nil
}

// generateValuesOverrides creates a map containing overrides for chart values based on the chart and component
// Specifically it merges DeployOpts.ValuesOverridesMap over Zarf `variables` for a given component/chart combination
func (p *Packager) generateValuesOverrides(chart types.ZarfChart, componentName string) (map[string]any, error) {
	valuesOverrides := make(map[string]any)
	chartOverrides := make(map[string]any)

	for _, variable := range chart.Variables {
		if setVar, ok := p.variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
			// Use the variable's path as a key to ensure unique entries for variables with the same name but different paths.
			if err := helpers.MergePathAndValueIntoMap(chartOverrides, variable.Path, setVar.Value); err != nil {
				return nil, fmt.Errorf("unable to merge path and value into map: %w", err)
			}
		}
	}

	// Apply any direct overrides specified in the deployment options for this component and chart
	if componentOverrides, ok := p.cfg.DeployOpts.ValuesOverridesMap[componentName]; ok {
		if chartSpecificOverrides, ok := componentOverrides[chart.Name]; ok {
			valuesOverrides = chartSpecificOverrides
		}
	}

	// Merge chartOverrides into valuesOverrides to ensure all overrides are applied.
	// This corrects the logic to ensure that chartOverrides and valuesOverrides are merged correctly.
	return helpers.MergeMapRecursive(chartOverrides, valuesOverrides), nil
}

// Install all Helm charts and raw k8s manifests into the k8s cluster.
func (p *Packager) installChartAndManifests(componentPaths *layout.ComponentPaths, component types.ZarfComponent) (installedCharts []types.InstalledChart, err error) {
	for _, chart := range component.Charts {
		// Do not wait for the chart to be ready if data injections are present.
		if len(component.DataInjections) > 0 {
			chart.NoWait = true
		}

		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			valueFilePath := helm.StandardValuesName(componentPaths.Values, chart, idx)
			if err := p.variableConfig.ReplaceTextTemplate(valueFilePath); err != nil {
				return installedCharts, err
			}
		}

		// Create a Helm values overrides map from set Zarf `variables` and DeployOpts library inputs
		// Values overrides are to be applied in order of Helm Chart Defaults -> Zarf `valuesFiles` -> Zarf `variables` -> DeployOpts overrides
		valuesOverrides, err := p.generateValuesOverrides(chart, component.Name)
		if err != nil {
			return installedCharts, err
		}

		helmCfg := helm.New(
			chart,
			componentPaths.Charts,
			componentPaths.Values,
			helm.WithDeployInfo(
				p.cfg,
				p.variableConfig,
				p.state,
				p.cluster,
				valuesOverrides,
				p.cfg.DeployOpts.Timeout,
				p.cfg.PkgOpts.Retries),
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
			if helpers.InvalidPath(filepath.Join(componentPaths.Manifests, manifest.Files[idx])) {
				// The path is likely invalid because of how we compose OCI components, add an index suffix to the filename
				manifest.Files[idx] = fmt.Sprintf("%s-%d.yaml", manifest.Name, idx)
				if helpers.InvalidPath(filepath.Join(componentPaths.Manifests, manifest.Files[idx])) {
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
				p.cfg,
				p.variableConfig,
				p.state,
				p.cluster,
				nil,
				p.cfg.DeployOpts.Timeout,
				p.cfg.PkgOpts.Retries),
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

func (p *Packager) printTablesForDeployment(ctx context.Context, componentsToDeploy []types.DeployedComponent) {

	// If not init config, print the application connection table
	if !p.cfg.Pkg.IsInitConfig() {
		message.PrintConnectStringTable(p.connectStrings)
	} else {
		if p.cluster != nil {
			// Grab a fresh copy of the state (if we are able) to print the most up-to-date version of the creds
			freshState, err := p.cluster.LoadZarfState(ctx)
			if err != nil {
				freshState = p.state
			}
			// otherwise, print the init config connection and passwords
			message.PrintCredentialTable(freshState, componentsToDeploy)
		}
	}
}

// ServiceInfoFromServiceURL takes a serviceURL and parses it to find the service info for connecting to the cluster. The string is expected to follow the following format:
// Example serviceURL: http://{SERVICE_NAME}.{NAMESPACE}.svc.cluster.local:{PORT}.
func serviceInfoFromServiceURL(serviceURL string) (string, string, int, error) {
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return "", "", 0, err
	}

	// Get the remote port from the serviceURL.
	remotePort, err := strconv.Atoi(parsedURL.Port())
	if err != nil {
		return "", "", 0, err
	}

	// Match hostname against local cluster service format.
	pattern, err := regexp.Compile(`^(?P<name>[^\.]+)\.(?P<namespace>[^\.]+)\.svc\.cluster\.local$`)
	if err != nil {
		return "", "", 0, err
	}
	get, err := helpers.MatchRegex(pattern, parsedURL.Hostname())

	// If incomplete match, return an error.
	if err != nil {
		return "", "", 0, err
	}
	return get("namespace"), get("name"), remotePort, nil
}
