// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"crypto"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/git"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/internal/packager/images"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/otiai10/copy"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
)

var (
	hpaModified    bool
	valueTemplate  template.Values
	connectStrings = make(types.ConnectStrings)
)

// Deploy attempts to deploy the given PackageConfig.
func (p *Packager) Deploy() error {
	message.Debug("packager.Deploy()")

	if err := p.loadZarfPkg(); err != nil {
		return fmt.Errorf("unable to load the Zarf Package: %w", err)
	}

	// Now that we have read the zarf.yaml, check the package kind
	if p.cfg.Pkg.Kind == "ZarfInitConfig" {
		p.cfg.IsInitConfig = true
	}

	// Confirm the overall package deployment
	if !p.confirmAction("Deploy", p.cfg.SBOMViewFiles) {
		return fmt.Errorf("deployment cancelled")
	}

	// Set variables and prompt if --confirm is not set
	if err := p.setActiveVariables(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	// Reset registry HPA scale down whether an error occurs or not
	defer func() {
		if p.cluster != nil && hpaModified {
			p.cluster.EnableRegHPAScaleDown()
		}
	}()

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := p.deployComponents()
	if err != nil {
		return fmt.Errorf("unable to deploy all components in this Zarf Package: %w", err)
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf deployment complete")
	p.printTablesForDeployment(deployedComponents)

	// Save deployed package information to k8s
	// Note: Not all packages need k8s; check if k8s is being used before saving the secret
	if p.cluster != nil {
		p.cluster.RecordPackageDeployment(p.cfg.Pkg, deployedComponents)
	}

	return nil
}

// deployComponents loops through a list of ZarfComponents and deploys them.
func (p *Packager) deployComponents() (deployedComponents []types.DeployedComponent, err error) {
	componentsToDeploy := p.getValidComponents()
	config.SetDeployingComponents(deployedComponents)

	// Generate a value template
	if valueTemplate, err = template.Generate(p.cfg); err != nil {
		return deployedComponents, fmt.Errorf("unable to generate the value template: %w", err)
	}

	for _, component := range componentsToDeploy {
		var charts []types.InstalledChart

		if p.cfg.IsInitConfig {
			charts, err = p.deployInitComponent(component)
		} else {
			charts, err = p.deployComponent(component, false /* keep img checksum */)
		}

		deployedComponent := types.DeployedComponent{Name: component.Name}
		onDeploy := component.Actions.OnDeploy

		onFailure := func() {
			if err := p.runActions(onDeploy.Defaults, onDeploy.OnFailure, &valueTemplate); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
			}
		}

		if err != nil {
			onFailure()
			return deployedComponents, fmt.Errorf("unable to deploy component %s: %w", component.Name, err)
		}

		if err := p.runActions(onDeploy.Defaults, onDeploy.OnSuccess, &valueTemplate); err != nil {
			onFailure()
			return deployedComponents, fmt.Errorf("unable to run component success action: %w", err)
		}

		// Deploy the component
		deployedComponent.InstalledCharts = charts
		deployedComponents = append(deployedComponents, deployedComponent)
		config.SetDeployingComponents(deployedComponents)
	}

	config.ClearDeployingComponents()
	return deployedComponents, nil
}

func (p *Packager) deployInitComponent(component types.ZarfComponent) (charts []types.InstalledChart, err error) {
	hasExternalRegistry := p.cfg.InitOpts.RegistryInfo.Address != ""
	isSeedRegistry := component.Name == "zarf-seed-registry"
	isRegistry := component.Name == "zarf-registry"
	isInjector := component.Name == "zarf-injector"
	isAgent := component.Name == "zarf-agent"

	// Always init the state on the seed registry component
	if isSeedRegistry {
		p.cluster, err = cluster.NewClusterWithWait(5 * time.Minute)
		if err != nil {
			return charts, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
		}
		p.cluster.InitZarfState(p.cfg.InitOpts)
	}

	if hasExternalRegistry && (isSeedRegistry || isInjector || isRegistry) {
		message.Notef("Not deploying the component (%s) since external registry information was provided during `zarf init`", component.Name)
		return charts, nil
	}

	// Before deploying the seed registry, start the injector
	if isSeedRegistry {
		p.cluster.StartInjectionMadness(p.tmp)
	}

	charts, err = p.deployComponent(component, isAgent /* skip img checksum if isAgent */)
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
func (p *Packager) deployComponent(component types.ZarfComponent, noImgChecksum bool) (charts []types.InstalledChart, err error) {
	message.Debugf("packager.deployComponent(%#v, %#v", p.tmp, component)

	// Toggles for general deploy operations
	componentPath, err := p.createComponentPaths(component)
	if err != nil {
		return charts, fmt.Errorf("unable to create the component paths: %w", err)
	}

	// All components now require a name
	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))

	hasImages := len(component.Images) > 0
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasDataInjections := len(component.DataInjections) > 0

	onDeploy := component.Actions.OnDeploy

	if err = p.runActions(onDeploy.Defaults, onDeploy.Before, &valueTemplate); err != nil {
		return charts, fmt.Errorf("unable to run component before action: %w", err)
	}

	if err := p.processComponentFiles(component, componentPath.Files); err != nil {
		return charts, fmt.Errorf("unable to process the component files: %w", err)
	}

	if !valueTemplate.Ready() && (hasImages || hasCharts || hasManifests || hasRepos) {

		// Make sure we have access to the cluster
		if p.cluster == nil {
			p.cluster, err = cluster.NewClusterWithWait(30 * time.Second)
			if err != nil {
				return charts, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
			}
		}

		// Disable the registry HPA scale down if we are deploying images and it is not already disabled
		if hasImages && !hpaModified && p.cfg.State.RegistryInfo.InternalRegistry {
			if err := p.cluster.DisableRegHPAScaleDown(); err != nil {
				message.Debugf("unable to toggle the registry HPA scale down: %s", err.Error())
			} else {
				hpaModified = true
			}
		}

		valueTemplate, err = p.getUpdatedValueTemplate(component)
		if err != nil {
			return charts, fmt.Errorf("unable to get the updated value template: %w", err)
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
		p.performDataInjections(&waitGroup, componentPath, component.DataInjections)
	}

	if hasCharts || hasManifests {
		if charts, err = p.installChartAndManifests(componentPath, component); err != nil {
			return charts, fmt.Errorf("unable to install helm chart(s): %w", err)
		}
	}

	if err = p.runActions(onDeploy.Defaults, onDeploy.After, &valueTemplate); err != nil {
		return charts, fmt.Errorf("unable to run component after action: %w", err)
	}

	return charts, nil
}

// Move files onto the host of the machine performing the deployment.
func (p *Packager) processComponentFiles(component types.ZarfComponent, sourceLocation string) error {
	// If there are no files to process, return early.
	if len(component.Files) < 1 {
		return nil
	}

	spinner := *message.NewProgressSpinner("Copying %d files", len(component.Files))
	defer spinner.Stop()

	for index, file := range component.Files {
		spinner.Updatef("Loading %s", file.Target)
		sourceFile := filepath.Join(sourceLocation, strconv.Itoa(index))

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			spinner.Updatef("Validating SHASUM for %s", file.Target)
			if shasum, _ := utils.GetCryptoHash(sourceFile, crypto.SHA256); shasum != file.Shasum {
				return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", file.Source, file.Shasum, shasum)
			}
		}

		// Replace temp target directories
		file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", p.tmp.Base, 1)

		// Check if the file looks like a text file
		isText, err := utils.IsTextFile(sourceFile)
		if err != nil {
			message.Debugf("unable to determine if file %s is a text file: %s", sourceFile, err)
		}

		// If the file is a text file, template it
		if isText {
			spinner.Updatef("Templating %s", file.Target)
			if err := valueTemplate.Apply(component, sourceFile, true); err != nil {
				return fmt.Errorf("unable to template file %s: %w", sourceFile, err)
			}
		}

		// Copy the file to the destination
		spinner.Updatef("Saving %s", file.Target)
		err = copy.Copy(sourceFile, file.Target)
		if err != nil {
			return fmt.Errorf("unable to copy file %s to %s: %w", sourceFile, file.Target, err)
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
		_ = os.RemoveAll(sourceFile)
	}

	spinner.Success()

	return nil
}

// Fetch the current ZarfState from the k8s cluster and generate a valueTemplate from the state values.
func (p *Packager) getUpdatedValueTemplate(component types.ZarfComponent) (values template.Values, err error) {
	// If we are touching K8s, make sure we can talk to it once per deployment
	spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
	defer spinner.Stop()

	state, err := p.cluster.LoadZarfState()
	// Return on error if we are not in YOLO mode
	if err != nil && !p.cfg.Pkg.Metadata.YOLO {
		return values, fmt.Errorf("unable to load the Zarf State from the Kubernetes cluster: %w", err)
	}

	// Check if the state is empty (uninitialized cluster)
	if state.Distro == "" {
		// If this is not a YOLO mode package, return an error
		if !p.cfg.Pkg.Metadata.YOLO {
			return values, fmt.Errorf("unable to load the Zarf State from the Kubernetes cluster: %w", err)
		}

		// YOLO mode, so minimal state needed
		state.Distro = "YOLO"

		// Try to create the zarf namespace
		spinner.Updatef("Creating the Zarf namespace")
		if _, err := p.cluster.Kube.CreateNamespace(cluster.ZarfNamespace, nil); err != nil {
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

	// Only check the architecture if the package has images
	if len(component.Images) > 0 && state.Architecture != p.arch {
		// If the package has images but the architectures don't match warn the user to avoid ugly hidden errors with image push/pull
		return values, fmt.Errorf("this package architecture is %s, but this cluster seems to be initialized with the %s architecture",
			state.Architecture, p.arch)
	}

	spinner.Success()
	return values, nil
}

// Push all of the components images to the configured container registry.
func (p *Packager) pushImagesToRegistry(componentImages []string, noImgChecksum bool) error {
	if len(componentImages) == 0 {
		return nil
	}

	imgConfig := images.ImgConfig{
		ImagesPath: p.tmp.Images,
		ImgList:    componentImages,
		NoChecksum: noImgChecksum,
		RegInfo:    p.cfg.State.RegistryInfo,
		Insecure:   config.CommonOptions.Insecure,
	}

	return utils.Retry(func() error {
		return imgConfig.PushToZarfRegistry()
	}, 3, 5*time.Second)
}

// Push all of the components git repos to the configured git server.
func (p *Packager) pushReposToRepository(reposPath string, repos []string) error {
	for _, repoURL := range repos {

		// Create an anonymous function to push the repo to the Zarf git server
		tryPush := func() error {
			gitClient := git.New(p.cfg.State.GitServer)
			svcInfo, err := cluster.ServiceInfoFromServiceURL(gitClient.Server.Address)

			// If this is a service (no error getting svcInfo), create a port-forward tunnel to that resource
			if err == nil {
				tunnel, err := cluster.NewTunnel(svcInfo.Namespace, cluster.SvcResource, svcInfo.Name, 0, svcInfo.Port)
				if err != nil {
					return err
				}

				tunnel.Connect("", false)
				defer tunnel.Close()
				gitClient.Server.Address = tunnel.HTTPEndpoint()
			}

			return gitClient.PushRepo(repoURL, reposPath)
		}

		// Try repo push up to 3 times
		if err := utils.Retry(tryPush, 3, 5*time.Second); err != nil {
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

	for _, data := range dataInjections {
		waitGroup.Add(1)
		go p.cluster.HandleDataInjection(waitGroup, data, componentPath)
	}
}

// Install all Helm charts and raw k8s manifests into the k8s cluster.
func (p *Packager) installChartAndManifests(componentPath types.ComponentPaths, component types.ZarfComponent) ([]types.InstalledChart, error) {
	installedCharts := []types.InstalledChart{}

	for _, chart := range component.Charts {
		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			chartValueName := helm.StandardName(componentPath.Values, chart) + "-" + strconv.Itoa(idx)
			if err := valueTemplate.Apply(component, chartValueName, false); err != nil {
				return installedCharts, err
			}
		}

		// Generate helm templates to pass to gitops engine
		helmCfg := &helm.Helm{
			BasePath:  componentPath.Base,
			Chart:     chart,
			Component: component,
			Cfg:       p.cfg,
			Cluster:   p.cluster,
		}

		addedConnectStrings, installedChartName, err := helmCfg.InstallOrUpgradeChart()
		if err != nil {
			return installedCharts, err
		}
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
		helmCfg := helm.Helm{
			BasePath:  componentPath.Manifests,
			Component: component,
			Cfg:       p.cfg,
			Cluster:   p.cluster,
		}
		addedConnectStrings, installedChartName, err := helmCfg.GenerateChart(manifest)
		if err != nil {
			return installedCharts, err
		}
		installedCharts = append(installedCharts, types.InstalledChart{Namespace: manifest.Namespace, ChartName: installedChartName})

		// Iterate over any connectStrings and add to the main map
		for name, description := range addedConnectStrings {
			connectStrings[name] = description
		}
	}

	return installedCharts, nil
}

func (p *Packager) printTablesForDeployment(componentsToDeploy []types.DeployedComponent) {
	pterm.Println()

	// If not init config, print the application connection table
	if !p.cfg.IsInitConfig {
		message.PrintConnectStringTable(connectStrings)
	} else {
		// otherwise, print the init config connection and passwords
		utils.PrintCredentialTable(p.cfg.State, componentsToDeploy)
	}
}
