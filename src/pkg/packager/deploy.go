// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
package packager

import (
	"bufio"
	"fmt"
	"io/ioutil"
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
	"github.com/defenseunicorns/zarf/src/internal/packager/kustomize"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/otiai10/copy"
	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"

	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var valueTemplate template.Values
var connectStrings = make(types.ConnectStrings)

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

	// If init config, make sure things are ready
	if p.cfg.IsInitConfig {
		utils.RunPreflightChecks()
	}

	// Confirm the overall package deployment
	if !p.confirmAction("Deploy", p.cfg.SBOMViewFiles) {
		return fmt.Errorf("deployment cancelled")
	}

	// Set variables and prompt if --confirm is not set
	if err := p.setActiveVariables(); err != nil {
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
		p.cluster.RecordPackageDeployment(p.cfg.Pkg, deployedComponents)
	}

	return nil
}

// deployComponents loops through a list of ZarfComponents and deploys them
func (p *Packager) deployComponents() (deployedComponents []types.DeployedComponent, err error) {
	componentsToDeploy := p.getValidComponents()
	config.SetDeployingComponents(deployedComponents)

	// Generate a value template
	valueTemplate, err = template.Generate(p.cfg)
	if err != nil {
		return deployedComponents, fmt.Errorf("unable to generate the value template: %w", err)
	}

	for _, component := range componentsToDeploy {
		var charts []types.InstalledChart

		deployedComponent := types.DeployedComponent{Name: component.Name}

		if p.cfg.IsInitConfig {
			charts, err = p.deployInitComponent(component)
		} else {
			charts, err = p.deployComponent(component, false /* keep img checksum */)
		}

		if err != nil {
			return deployedComponents, fmt.Errorf("unable to deploy component %s: %w", component.Name, err)
		}

		// Deploy the component
		deployedComponent.InstalledCharts = charts
		deployedComponents = append(deployedComponents, deployedComponent)
		config.SetDeployingComponents(deployedComponents)
	}

	// config.ClearDeployingComponents()
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
		p.cluster.InitZarfState(p.tmp, p.cfg.InitOpts)
	}

	if hasExternalRegistry && (isSeedRegistry || isInjector || isRegistry) {
		message.Notef("Not deploying the component (%s) since external registry information was provided during `zarf init`", component.Name)
		return charts, nil
	}

	// Before deploying the seed registry, start the injector
	if isSeedRegistry {
		p.cluster.RunInjectionMadness(p.tmp)
	}

	charts, err = p.deployComponent(component, isAgent /* skip img checksum if isAgent */)
	if err != nil {
		return charts, fmt.Errorf("unable to deploy component %s: %w", component.Name, err)
	}

	// Do cleanup for when we inject the seed registry during initialization
	if isSeedRegistry {
		err := p.cluster.PostSeedRegistry(p.tmp)
		if err != nil {
			return charts, fmt.Errorf("unable to seed the Zarf Registry: %w", err)
		}

		seedImage := fmt.Sprintf("%s:%s", config.ZarfSeedImage, config.ZarfSeedTag)
		imgConfig := images.ImgConfig{
			TarballPath: p.tmp.SeedImage,
			ImgList:     []string{seedImage},
			NoChecksum:  true,
			RegInfo:     p.cfg.State.RegistryInfo,
		}

		// Push the seed images into to Zarf registry
		if err = imgConfig.PushToZarfRegistry(); err != nil {
			return charts, fmt.Errorf("unable to push the seed images to the Zarf Registry: %w", err)
		}
	}

	return charts, nil
}

// Deploy a Zarf Component
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
	hasBigBang := component.BigBang.Version != ""

	// Run the 'before' scripts and move files before we do anything else
	if err = p.runComponentScripts(component.Scripts.Before, component.Scripts); err != nil {
		return charts, fmt.Errorf("unable to run the 'before' scripts: %w", err)
	}

	if err := p.processComponentFiles(component.Files, componentPath.Files); err != nil {
		return charts, fmt.Errorf("unable to process the component files: %w", err)
	}

	if !valueTemplate.Ready() && (hasImages || hasCharts || hasManifests || hasRepos || hasBigBang) {

		// Make sure we have access to the cluster
		if p.cluster == nil {
			p.cluster, err = cluster.NewClusterWithWait(30 * time.Second)
			if err != nil {
				return charts, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
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

	if hasBigBang {
		if _, err := p.installBigBang(componentPath, component); err != nil {
			return []types.InstalledChart{
				{
					Namespace: "bigbang",
					ChartName: "bigbang",
				},
			}, fmt.Errorf("unable to install big Bang: %w", err)
		}
	}

	// Run the 'after' scripts after all other attributes of the component has been deployed
	p.runComponentScripts(component.Scripts.After, component.Scripts)

	return charts, nil
}

// Move files onto the host of the machine performing the deployment
func (p *Packager) processComponentFiles(componentFiles []types.ZarfFile, sourceLocation string) error {
	// If there are no files to process, return early.
	if len(componentFiles) < 1 {
		return nil
	}

	spinner := *message.NewProgressSpinner("Copying %d files", len(componentFiles))
	defer spinner.Stop()

	for index, file := range componentFiles {
		spinner.Updatef("Loading %s", file.Target)
		sourceFile := filepath.Join(sourceLocation, strconv.Itoa(index))

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			spinner.Updatef("Validating SHASUM for %s", file.Target)
			if shasum, _ := utils.GetSha256Sum(sourceFile); shasum != file.Shasum {
				return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s", file.Source, file.Shasum, shasum)
			}
		}

		// Replace temp target directories
		file.Target = strings.Replace(file.Target, "###ZARF_TEMP###", p.tmp.Base, 1)

		// Copy the file to the destination
		spinner.Updatef("Saving %s", file.Target)
		err := copy.Copy(sourceFile, file.Target)
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

// Fetch the current ZarfState from the k8s cluster and generate a valueTemplate from the state values
func (p *Packager) getUpdatedValueTemplate(component types.ZarfComponent) (values template.Values, err error) {
	// If we are touching K8s, make sure we can talk to it once per deployment
	spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
	defer spinner.Stop()

	state, err := p.cluster.LoadZarfState()

	// If no distro the zarf secret did not load properly
	if err != nil || state.Distro == "" {
		return values, err
	}

	p.cfg.State = state

	// Continue loading state data if it is valid
	values, err = template.Generate(p.cfg)
	if err != nil {
		return values, err
	}

	if len(component.Images) > 0 && state.Architecture != p.arch {
		// If the package has images but the architectures don't match warn the user to avoid ugly hidden errors with image push/pull
		return values, fmt.Errorf("this package architecture is %s, but this cluster seems to be initialized with the %s architecture",
			state.Architecture, p.arch)
	}

	spinner.Success()
	return values, nil
}

// Push all of the components images to the configured container registry
func (p *Packager) pushImagesToRegistry(componentImages []string, noImgChecksum bool) error {
	if len(componentImages) == 0 {
		return nil
	}

	imgConfig := images.ImgConfig{
		TarballPath: p.tmp.Images,
		ImgList:     componentImages,
		NoChecksum:  noImgChecksum,
		RegInfo:     p.cfg.State.RegistryInfo,
	}

	return utils.Retry(func() error {
		return imgConfig.PushToZarfRegistry()
	}, 3, 5*time.Second)
}

// Push all of the components git repos to the configured git server
func (p *Packager) pushReposToRepository(reposPath string, repos []string) error {
	for _, repoURL := range repos {

		// Create an anonymous function to push the repo to the Zarf git server
		tryPush := func() error {
			gitClient := git.New(p.cfg.State.GitServer)

			// If this is a serviceURL, create a port-forward tunnel to that resource
			if cluster.IsServiceURL(gitClient.Server.Address) {
				if tunnel, err := cluster.NewTunnelFromServiceURL(gitClient.Server.Address); err != nil {
					return err
				} else {
					tunnel.Connect("", false)
					defer tunnel.Close()
					gitClient.Server.Address = fmt.Sprintf("http://%s", tunnel.Endpoint())
				}
			}

			// Convert the repo URL to a Zarf-formatted repo name
			if repoPath, err := gitClient.TransformURLtoRepoName(repoURL); err != nil {
				return fmt.Errorf("unable to get the repo name from the URL %s: %w", repoURL, err)
			} else {
				return gitClient.PushRepo(filepath.Join(reposPath, repoPath))
			}
		}

		// Try repo push up to 3 times
		if err := utils.Retry(tryPush, 3, 5*time.Second); err != nil {
			return fmt.Errorf("unable to push repo %s to the Git Server: %w", repoURL, err)
		}
	}

	return nil
}

// Async'ly move data into a container running in a pod on the k8s cluster
func (p *Packager) performDataInjections(waitGroup *sync.WaitGroup, componentPath types.ComponentPaths, dataInjections []types.ZarfDataInjection) {
	if len(dataInjections) > 0 {
		message.Info("Loading data injections")
	}

	for _, data := range dataInjections {
		waitGroup.Add(1)
		go p.cluster.HandleDataInjection(waitGroup, data, componentPath)
	}
}

// Install all Helm charts and raw k8s manifests into the k8s cluster
func (p *Packager) installChartAndManifests(componentPath types.ComponentPaths, component types.ZarfComponent) ([]types.InstalledChart, error) {
	installedCharts := []types.InstalledChart{}

	for _, chart := range component.Charts {
		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			chartValueName := helm.StandardName(componentPath.Values, chart) + "-" + strconv.Itoa(idx)
			valueTemplate.Apply(component, chartValueName)
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

func (p *Packager) installBigBang(componentPath types.ComponentPaths, component types.ZarfComponent) ([]types.InstalledChart, error) {
	installedCharts := []types.InstalledChart{}
	var err error
	// need to upload images and repos

	//read the images.txt file in the files
	imageFile := fmt.Sprintf("%s/images.txt", componentPath.Files)
	reposFile := fmt.Sprintf("%s/repos.txt", componentPath.Files)

	// file, err := os.Open("imageFile.txt")

	if err != nil {
		return installedCharts, err

	}
	file, err := os.Open(imageFile)
	// The bufio.NewScanner() function is called in which the
	// object os.File passed as its parameter and this returns a
	// object bufio.Scanner which is further used on the
	// bufio.Scanner.Split() method.
	scanner := bufio.NewScanner(file)

	// The bufio.ScanLines is used as an
	// input to the method bufio.Scanner.Split()
	// and then the scanning forwards to each
	// new line using the bufio.Scanner.Scan()
	// method.
	scanner.Split(bufio.ScanLines)
	var images []string

	for scanner.Scan() {
		images = append(images, scanner.Text())
	}

	// The method os.File.Close() is called
	// on the os.File object to close the file
	file.Close()

	if err := p.pushImagesToRegistry(images, false); err != nil {
		return installedCharts, fmt.Errorf("unable to push images to the registry: %w", err)
	}

	file, err = os.Open(reposFile)
	//repos
	scanner = bufio.NewScanner(file)

	scanner.Split(bufio.ScanLines)
	var repos []string

	for scanner.Scan() {
		repos = append(repos, scanner.Text())
	}

	// The method os.File.Close() is called
	// on the os.File object to close the file
	file.Close()

	if err = p.pushReposToRepository(componentPath.Repos, repos); err != nil {
		return installedCharts, fmt.Errorf("unable to push the repos to the repository: %w", err)
	}

	fmt.Printf("Deploying Big Bang\n")
	if component.BigBang.DeployFlux {
		fmt.Printf("Deploying Flux\n")
		manifest := types.ZarfManifest{
			Namespace: "flux-system",
			Name:      "flux-system",
			Files: []string{
				// i know what it is b/c I'm smart
				"kustomization-flux-system-0.yaml",
			},
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

	// now how does this look:
	// we have a list of valuesFiles

	// 1. Create a new Kustomization that
	// creates a secret per file

	//"sigs.k8s.io/kustomize/api/krusty"
	// kustypes "sigs.k8s.io/kustomize/api/types"
	chart := types.ZarfChart{
		Name:        "bigbang",
		Url:         "https://repo1.dso.mil/platform-one/big-bang/bigbang.git",
		Version:     component.BigBang.Version,
		ValuesFiles: component.BigBang.ValuesFrom,
		GitPath:     "./chart",
	}
	if component.BigBang.Repo != "" {
		chart.Url = component.BigBang.Repo
	}
	// Iterate over any connectStrings and add to the main map
	helmCfg := helm.Helm{
		BasePath:  componentPath.Manifests,
		Component: component,
		Cfg:       p.cfg,
		Cluster:   p.cluster,
	}

	bb := kustypes.Kustomization{
		Resources: []string{
			fmt.Sprintf("%s/base?ref=%v", "git::https://repo1.dsop.io/platform-one/big-bang/bigbang.git", component.BigBang.Version),
		},
		SecretGenerator: make([]kustypes.SecretArgs, len(component.BigBang.ValuesFrom)+1),
		PatchesJson6902: make([]kustypes.Patch, 1),
	}

	// write the kustomization file on disk
	os.Mkdir(fmt.Sprintf("%s/bigbang", componentPath.Manifests), 0700)

	// zarf magic for the value file
	for i := range component.BigBang.ValuesFrom {
		destination := fmt.Sprintf("%s/bigbang/%s", componentPath.Manifests, component.BigBang.ValuesFrom[i])
		if err := utils.CreatePathAndCopy(component.BigBang.ValuesFrom[i], destination); err != nil {
			return installedCharts, fmt.Errorf("unable to copy manifest %s: %w", component.BigBang.ValuesFrom[i], err)
		}
		chartValueName := helm.StandardName(componentPath.Values, chart) + "-" + strconv.Itoa(i)
		valueTemplate.Apply(component, chartValueName)
		if err := utils.CreatePathAndCopy(component.BigBang.ValuesFrom[i], destination); err != nil {
			return installedCharts, fmt.Errorf("unable to copy manifest %s: %w", component.BigBang.ValuesFrom[i], err)
		}
		// copy the file from the
		secret := kustypes.SecretArgs{
			GeneratorArgs: kustypes.GeneratorArgs{
				Name:     fmt.Sprintf("values-%d", i),
				Behavior: kustypes.BehaviorCreate.String(),
				KvPairSources: kustypes.KvPairSources{
					FileSources: []string{fmt.Sprintf("values.yaml=%s", destination)},
				},
			},
		}
		bb.SecretGenerator[i] = secret
	}
	// Think this is all we need for zarf specific things, but could add more
	creds := `
registryCredentials:
  registry: "###ZARF_REGISTRY###"
  username: "zarf-pull"
  password: "###ZARF_REGISTRY_AUTH_PULL###"
git:
  existingSecret: "private-git-server"
`
	ioutil.WriteFile(fmt.Sprintf("%s/bigbang/zarf-credentials.yaml", componentPath.Manifests), []byte(creds), 0700)
	//Zarf Render
	valueTemplate.Apply(component, fmt.Sprintf("%s/bigbang/zarf-credentials.yaml", componentPath.Manifests))

	bb.SecretGenerator[len(component.BigBang.ValuesFrom)] = kustypes.SecretArgs{
		GeneratorArgs: kustypes.GeneratorArgs{
			Name:     fmt.Sprintf("values-zarf-registry"),
			Behavior: kustypes.BehaviorCreate.String(),
			KvPairSources: kustypes.KvPairSources{
				FileSources: []string{
					"values.yaml=zarf-credentials.yaml",
				},
			},
		},
	}
	// bb.Transformers = []string{}
	bb.GeneratorOptions = &kustypes.GeneratorOptions{
		DisableNameSuffixHash: true,
	}
	bb.PatchesJson6902 = []kustypes.Patch{
		{
			Target: &kustypes.Selector{
				ResId: resid.ResId{
					Name:      "bigbang",
					Namespace: "bigbang",
					Gvk: resid.Gvk{
						Group:   "helm.toolkit.fluxcd.io",
						Kind:    "HelmRelease",
						Version: "v2beta1",
					},
				},
			},

			// Hard code for now.  This allows for only 1 values file to be used until
			// we dynamically generate this patch
			Patch: `
- op: add
  path: /spec/valuesFrom/-
  value:
    kind: Secret
    name: values-0
- op: add
  path: /spec/valuesFrom/-
  value:
    kind: Secret
    name: values-zarf-registry
`,
		},
	}
	b, _ := yaml.Marshal(bb)
	// write the kustomization file on disk
	os.Mkdir(fmt.Sprintf("%s/bigbang", componentPath.Manifests), 0700)
	d1 := fmt.Sprintf("%s/bigbang/kustomization.yaml", componentPath.Manifests)
	ioutil.WriteFile(d1, b, 0700)

	//Zarf Render the variables out of things
	valueTemplate.Apply(component, d1)

	// render the kustomization to a bunch of objects
	fmt.Printf("MANFIEST PATH: %v\n", componentPath.Manifests)
	destination := fmt.Sprintf("%s/%s", componentPath.Manifests, "kustomization-bigbang.yaml")

	// This wont work on the airgap since this kustomization rendering happens in the deploy phase
	if err := kustomize.BuildKustomization(fmt.Sprintf("%s/bigbang/", componentPath.Manifests), destination, true); err != nil {
		return installedCharts, fmt.Errorf("unable to build kustomization %s: %w", "bigbang", err)
	}

	// deploy the objects

	// patches the HelmRelease to add each secret as a spec.valuesFrom
	// Should look like this Kustomization:
	/*
		resources:
		  - git::https://repo1.dsop.io/platform-one/big-bang/bigbang.git/base?ref=1.47.0
		secretGenerator:
		  - name: values-1
		    behavior: create
		    files:
		      - values.yaml=values.yaml
		  - name: values-2
		    behavior: create
		    files:
		      - values.yaml=values2.yaml
		patchesJson6902:
		- target:
		    group: helm.toolkit.fluxcd.io
		    version: v2beta1
		    kind: HelmRelease
		    name: bigbang
		  patch: |-
		    - op: add
		      path: /spec/valuesFrom/-
		      value:
		        kind: Secret
		        name: values-1
		    - op: add
		      path: /spec/valuesFrom/-
		      value:
		        kind: Secret
		        name: values-2
	*/

	//XXX debugging, delete later
	objects, _ := ioutil.ReadFile("kustomization-bigbang.yaml")
	fmt.Printf("BIG BANG!!!\n%s\n", string(objects))
	k := types.ZarfManifest{
		Name:      "bigbang",
		Namespace: "bigbang",
		Files: []string{
			// destination,
			"kustomization-bigbang.yaml",
		},
	}
	fmt.Printf("Deploying Flux\n")

	// Iterate over any connectStrings and add to the main map

	addedConnectStrings, installedChartName, err := helmCfg.GenerateChart(k)
	if err != nil {
		return installedCharts, err
	}
	installedCharts = append(installedCharts, types.InstalledChart{Namespace: k.Namespace, ChartName: installedChartName})

	// Iterate over any connectStrings and add to the main map
	for name, description := range addedConnectStrings {
		connectStrings[name] = description
	}

	return installedCharts, err

}

func (p *Packager) printTablesForDeployment(componentsToDeploy []types.DeployedComponent) {
	pterm.Println()

	// If not init config, print the application connection table
	if !p.cfg.IsInitConfig {
		message.PrintConnectStringTable(connectStrings)
	} else {
		// otherwise, print the init config connection and passwords
		loginTableHeader := pterm.TableData{
			{"     Application", "Username", "Password", "Connect"},
		}

		loginTable := pterm.TableData{}
		if p.cfg.State.RegistryInfo.InternalRegistry {
			loginTable = append(loginTable, pterm.TableData{{"     Registry", p.cfg.State.RegistryInfo.PushUsername, p.cfg.State.RegistryInfo.PushPassword, "zarf connect registry"}}...)
		}

		for _, component := range componentsToDeploy {
			// Show message if including logging stack
			if component.Name == "logging" {
				loginTable = append(loginTable, pterm.TableData{{"     Logging", "zarf-admin", p.cfg.State.LoggingSecret, "zarf connect logging"}}...)
			}
			// Show message if including git-server
			if component.Name == "git-server" {
				loginTable = append(loginTable, pterm.TableData{
					{"     Git", p.cfg.State.GitServer.PushUsername, p.cfg.State.GitServer.PushPassword, "zarf connect git"},
					{"     Git (read-only)", p.cfg.State.GitServer.PullUsername, p.cfg.State.GitServer.PullPassword, "zarf connect git"},
				}...)
			}
		}

		if len(loginTable) > 0 {
			loginTable = append(loginTableHeader, loginTable...)
			_ = pterm.DefaultTable.WithHasHeader().WithData(loginTable).Render()
		}
	}
}
