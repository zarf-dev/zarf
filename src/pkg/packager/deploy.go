// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/actions"
	"github.com/defenseunicorns/zarf/src/internal/packager/files"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/internal/packager/variables"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
)

// Deploy attempts to deploy the given PackageConfig.
func (p *Packager) Deploy() (err error) {
	// Attempt to connect to a Kubernetes cluster.
	// Not all packages require Kubernetes, so we only want to log a debug message rather than return the error when we can't connect to a cluster.
	p.cluster, err = cluster.NewCluster()
	if err != nil {
		message.Debug(err)
	}

	if helpers.IsOCIURL(p.cfg.PkgOpts.PackagePath) {
		err := p.SetOCIRemote(p.cfg.PkgOpts.PackagePath)
		if err != nil {
			return err
		}
	}

	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the Zarf Package: %w", err)
	}

	if err := p.validatePackageArchitecture(); err != nil {
		if errors.Is(err, lang.ErrUnableToCheckArch) {
			message.Warnf("Unable to validate package architecture: %s", err.Error())
		} else {
			return err
		}
	}

	if err := ValidatePackageSignature(p.tmp.Base, p.cfg.PkgOpts.PublicKeyPath); err != nil {
		return err
	}

	if err := p.validateLastNonBreakingVersion(); err != nil {
		return err
	}

	// Now that we have read the zarf.yaml, check the package kind
	if p.cfg.Pkg.Kind == types.ZarfInitConfig {
		p.cfg.IsInitConfig = true
	}

	// Confirm the overall package deployment
	if !p.confirmAction(config.ZarfDeployStage, p.cfg.SBOMViewFiles) {
		return fmt.Errorf("deployment cancelled")
	}

	// Set variables and prompt if --confirm is not set
	if p.valueTemplate, err = variables.New(p.cfg.Pkg, p.cfg.PkgOpts.SetVariables); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	p.hpaModified = false
	p.connectStrings = make(types.ConnectStrings)
	// Reset registry HPA scale down whether an error occurs or not
	defer func() {
		if p.cluster != nil && p.hpaModified {
			if err := p.cluster.EnableRegHPAScaleDown(); err != nil {
				message.Debugf("unable to reenable the registry HPA scale down: %s", err.Error())
			}
		}
	}()

	// Filter out components that are not compatible with this system
	p.filterComponents(true)

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents()
	if err != nil {
		return fmt.Errorf("unable to deploy all components in this Zarf Package: %w", err)
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf deployment complete")

	p.printTablesForDeployment(deployedComponents)

	return nil
}

// deployComponents loops through a list of ZarfComponents and deploys them.
func (p *Packager) deployComponents() (deployedComponents []types.DeployedComponent, err error) {
	componentsToDeploy := p.getValidComponents()

	// Generate a variables value template
	if err = p.valueTemplate.WithState(p.cfg.State); err != nil {
		return deployedComponents, fmt.Errorf("unable to generate the value template: %w", err)
	}

	for _, component := range componentsToDeploy {
		var charts []types.InstalledChart

		if p.cfg.IsInitConfig {
			charts, err = p.deployInitComponent(component)
		} else {
			charts, err = p.deployComponent(component, false /* keep img checksum */, false /* always push images */)
		}

		deployedComponent := types.DeployedComponent{Name: component.Name}
		onDeploy := component.Actions.OnDeploy

		onFailure := func() {
			if err := actions.Run(onDeploy.Defaults, onDeploy.OnFailure, p.valueTemplate); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}
		if err != nil {
			onFailure()
			return deployedComponents, fmt.Errorf("unable to deploy component %s: %w", component.Name, err)
		}

		// Deploy the component
		deployedComponent.InstalledCharts = charts
		deployedComponents = append(deployedComponents, deployedComponent)

		// Save deployed package information to k8s
		// Note: Not all packages need k8s; check if k8s is being used before saving the secret
		if p.cluster != nil {
			err = p.cluster.RecordPackageDeployment(p.cfg.Pkg, deployedComponents, p.connectStrings)
			if err != nil {
				message.Debugf("Unable to record package deployment for component %s: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
			}
		}

		if err := actions.Run(onDeploy.Defaults, onDeploy.OnSuccess, p.valueTemplate); err != nil {
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
		p.cluster, err = cluster.NewClusterWithWait(5*time.Minute, true)
		if err != nil {
			return charts, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
		}

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
		p.cluster.StartInjectionMadness(p.tmp, component.Images)
	}

	charts, err = p.deployComponent(component, isAgent /* skip img checksum if isAgent */, isSeedRegistry /* skip image push if isSeedRegistry */)
	if err != nil {
		return charts, fmt.Errorf("unable to deploy component %s: %w", component.Name, err)
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
	componentPaths, err := p.createOrGetComponentPaths(component)
	if err != nil {
		return charts, fmt.Errorf("unable to create the component paths: %w", err)
	}

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	hasImages := len(component.Images) > 0 && !noImgPush
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasDataInjections := len(component.DataInjections) > 0

	onDeploy := component.Actions.OnDeploy

	if err = actions.Run(onDeploy.Defaults, onDeploy.Before, p.valueTemplate); err != nil {
		return charts, fmt.Errorf("unable to run component before action: %w", err)
	}

	// If there are no files to process, return early.
	// if len(f.component.Files) < 1 {
	// 	return nil
	// }

	// spinner := message.NewProgressSpinner("Copying %d files", len(f.component.Files))
	// defer spinner.Stop()

	for fileIdx, file := range component.Files {
		f := files.New(&file, strconv.Itoa(fileIdx), &component, componentPaths).WithValues(p.valueTemplate)

		if err := f.ProcessFile(); err != nil {
			return charts, fmt.Errorf("unable to process the component files: %w", err)
		}
	}

	if !p.valueTemplate.HasState() && p.requiresCluster(component) {
		// Make sure we have access to the cluster
		if p.cluster == nil {
			p.cluster, err = cluster.NewClusterWithWait(cluster.DefaultTimeout, true)
			if err != nil {
				return charts, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
			}
		}
		// Setup the state in the config and get the valuesTemplate
		err = p.setupState(component)
		if err != nil {
			return charts, fmt.Errorf("unable to get the updated value template: %w", err)
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
		if err = p.pushReposToRepository(componentPaths.Repos, component.Repos); err != nil {
			return charts, fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	if hasDataInjections {
		waitGroup := sync.WaitGroup{}
		defer waitGroup.Wait()
		p.performDataInjections(&waitGroup, componentPaths, component.DataInjections)
	}

	if hasCharts || hasManifests {
		if charts, err = p.installChartAndManifests(componentPaths, component); err != nil {
			return charts, fmt.Errorf("unable to install helm chart(s): %w", err)
		}
	}

	if err = actions.Run(onDeploy.Defaults, onDeploy.After, p.valueTemplate); err != nil {
		return charts, fmt.Errorf("unable to run component after action: %w", err)
	}

	return charts, nil
}

// Fetch the current ZarfState from the k8s cluster and generate a p.valueTemplate from the state values.
func (p *Packager) setupState(component types.ZarfComponent) (err error) {
	// If we are touching K8s, make sure we can talk to it once per deployment
	spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
	defer spinner.Stop()

	state, err := p.cluster.LoadZarfState()
	// Return on error if we are not in YOLO mode
	if err != nil && !p.cfg.Pkg.Metadata.YOLO {
		return fmt.Errorf("unable to load the Zarf State from the Kubernetes cluster: %w", err)
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
	err = p.valueTemplate.WithState(p.cfg.State)
	if err != nil {
		return err
	}

	// Only check the architecture if the package has images
	if len(component.Images) > 0 && state.Architecture != p.arch {
		// If the package has images but the architectures don't match, fail the deployment and warn the user to avoid ugly hidden errors with image push/pull
		return fmt.Errorf("this package architecture is %s, but this cluster seems to be initialized with the %s architecture",
			p.arch, state.Architecture)
	}

	spinner.Success()
	return nil
}

// Push all of the components images to the configured container registry.
func (p *Packager) pushImagesToRegistry(componentImages []string, noImgChecksum bool) error {
	if len(componentImages) == 0 {
		return nil
	}

	imgConfig := images.ImgConfig{
		ImagesPath:    p.tmp.Images,
		ImgList:       componentImages,
		NoChecksum:    noImgChecksum,
		RegInfo:       p.cfg.State.RegistryInfo,
		Insecure:      config.CommonOptions.Insecure,
		Architectures: []string{p.cfg.Pkg.Metadata.Architecture, p.cfg.Pkg.Build.Architecture},
	}

	return helpers.Retry(func() error {
		return imgConfig.PushToZarfRegistry()
	}, 3, 5*time.Second)
}

// Push all of the components git repos to the configured git server.
func (p *Packager) pushReposToRepository(reposPath string, repos []string) error {
	for _, repoURL := range repos {
		// Create an anonymous function to push the repo to the Zarf git server
		tryPush := func() error {
			gitConnectionInfo := p.cfg.State.GitServer
			svcInfo, err := cluster.ServiceInfoFromServiceURL(gitConnectionInfo.Address)

			// If this is a service (no error getting svcInfo), create a port-forward tunnel to that resource
			if err == nil {
				tunnel, err := cluster.NewTunnel(svcInfo.Namespace, cluster.SvcResource, svcInfo.Name, 0, svcInfo.Port)
				if err != nil {
					return err
				}

				err = tunnel.Connect("", false)
				if err != nil {
					return err
				}
				defer tunnel.Close()
				gitConnectionInfo.Address = tunnel.HTTPEndpoint()
			}

			gitClient := git.New(gitConnectionInfo)

			return gitClient.PushRepo(repoURL, reposPath)
		}

		// Try repo push up to 3 times
		if err := helpers.Retry(tryPush, 3, 5*time.Second); err != nil {
			return fmt.Errorf("unable to push repo %s to the Git Server: %w", repoURL, err)
		}
	}

	return nil
}

// Async move data into a container running in a pod on the k8s cluster.
func (p *Packager) performDataInjections(waitGroup *sync.WaitGroup, componentPath types.ComponentPaths, dataInjections []types.ZarfDataInjection) {
	if len(dataInjections) > 0 {
		message.Info("Loading data injections")
	}

	for idx, data := range dataInjections {
		waitGroup.Add(1)
		go p.cluster.HandleDataInjection(waitGroup, data, componentPath, idx)
	}
}

// Install all Helm charts and raw k8s manifests into the k8s cluster.
func (p *Packager) installChartAndManifests(componentPaths types.ComponentPaths, component types.ZarfComponent) (installedCharts []types.InstalledChart, err error) {
	for _, chart := range component.Charts {

		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			chartValueName := fmt.Sprintf("%s-%d", helm.StandardName(componentPaths.Values, &chart), idx)
			if err := p.valueTemplate.Apply(component, chartValueName); err != nil {
				return installedCharts, err
			}
		}

		helmCfg := helm.New(
			&chart, "",
		).WithCluster(
			p.cluster, p.cfg.State,
		).WithComponent(
			p.cfg.Pkg.Metadata, component, componentPaths,
		).WithValues(
			p.valueTemplate, p.cfg.DeployOpts.AdoptExistingResources,
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

		// Iterate over any connectStrings and add to the main map
		helmCfg := helm.New(
			nil, "",
		).WithCluster(
			p.cluster, p.cfg.State,
		).WithComponent(
			p.cfg.Pkg.Metadata, component, componentPaths,
		).WithValues(
			p.valueTemplate, p.cfg.DeployOpts.AdoptExistingResources,
		)

		// Generate the chart.
		if err := helmCfg.GenerateChart(manifest); err != nil {
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
	pterm.Println()

	// If not init config, print the application connection table
	if !p.cfg.IsInitConfig {
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
