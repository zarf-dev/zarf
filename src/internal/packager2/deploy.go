package packager2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/deprecated"
	"github.com/zarf-dev/zarf/src/pkg/pki"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeployOpts struct {
	cluster.InitStateOptions
	// Deploy time set variables
	SetVariables map[string]string
	// Whether to adopt any pre-existing K8s resources into the Helm charts managed by Zarf
	AdoptExistingResources bool
	// Timeout for performing Helm operations
	Timeout time.Duration
	// Retries to preform for operations like git and image pushes
	Retries int
	// [Library Only] A map of component names to chart names containing Helm Chart values to override values on deploy
	ValuesOverridesMap    map[string]map[string]map[string]interface{}
	OCIConcurrency        int
	PlainHTTP             bool
	InsecureTLSSkipVerify bool
}

// deployer tracks mutable fields across deployments. Because components can create a cluster and create state
// any of these fields are subject to change from one component to the next
type deployer struct {
	s           *state.State
	c           *cluster.Cluster
	vc          *variables.VariableConfig
	hpaModified bool
}

func Deploy(ctx context.Context, pkgLayout *layout.PackageLayout, opts DeployOpts) error {
	l := logger.From(ctx)
	l.Info("starting deploy")
	start := time.Now()
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkgLayout.Pkg.Constants)
	if err := variableConfig.PopulateVariables(pkgLayout.Pkg.Variables, opts.SetVariables); err != nil {
		return fmt.Errorf("unable to populate variables: %w", err)
	}

	if pkgLayout.Pkg.IsInitConfig() {
		for _, component := range pkgLayout.Pkg.Components {
			if component.Name == "k3s" {
				opts.ApplianceMode = true
			}
		}
	}

	d := deployer{
		vc:          variableConfig,
		hpaModified: false,
	}

	// During deploy we disable
	defer d.resetRegistryHPA(ctx)
	l.Debug("variables populated", "time", time.Since(start))

	deployedComponents, err := d.deployComponents(ctx, pkgLayout, opts)
	if err != nil {
		return err
	}
	//FIXME
	fmt.Println(deployedComponents)
	return nil
}

func (d *deployer) resetRegistryHPA(ctx context.Context) {
	l := logger.From(ctx)
	if d.c != nil && d.hpaModified {
		if err := d.c.EnableRegHPAScaleDown(ctx); err != nil {
			l.Debug("unable to reenable the registry HPA scale down", "error", err.Error())
		}
	}
}

func (d *deployer) isConnectedToCluster() bool {
	return d.c != nil
}

func (d *deployer) deployComponents(ctx context.Context, pkgLayout *layout.PackageLayout, opts DeployOpts) ([]types.DeployedComponent, error) {
	l := logger.From(ctx)
	deployedComponents := []types.DeployedComponent{}

	// Process all the components we are deploying
	for _, component := range pkgLayout.Pkg.Components {
		packageGeneration := 1
		// Connect to cluster if a component requires it.
		if component.RequiresCluster() {
			timeout := cluster.DefaultTimeout
			if pkgLayout.Pkg.IsInitConfig() {
				timeout = 5 * time.Minute
			}
			connectCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if !d.isConnectedToCluster() {
				if err := d.connectToCluster(connectCtx, pkgLayout.Pkg); err != nil {
					return nil, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
				}
			}
			// If this package has been deployed before, increment the package generation within the secret
			if existingDeployedPackage, _ := d.c.GetDeployedPackage(ctx, pkgLayout.Pkg.Metadata.Name); existingDeployedPackage != nil {
				packageGeneration = existingDeployedPackage.Generation + 1
			}
		}

		deployedComponent := types.DeployedComponent{
			Name:               component.Name,
			Status:             types.ComponentStatusDeploying,
			ObservedGeneration: packageGeneration,
		}

		// Ensure we don't overwrite any installedCharts data when updating the package secret
		if d.isConnectedToCluster() {
			installedCharts, err := d.c.GetInstalledChartsForComponent(ctx, pkgLayout.Pkg.Metadata.Name, component)
			if err != nil {
				l.Debug("unable to fetch installed Helm charts", "component", component.Name, "error", err.Error())
			}
			deployedComponent.InstalledCharts = installedCharts
		}

		deployedComponents = append(deployedComponents, deployedComponent)
		idx := len(deployedComponents) - 1
		if d.isConnectedToCluster() {
			if _, err := d.c.RecordPackageDeployment(ctx, pkgLayout.Pkg, deployedComponents, packageGeneration); err != nil {
				l.Debug("unable to record package deployment", "component", component.Name, "error", err.Error())
			}
		}
		// Deploy the component
		var charts []types.InstalledChart
		var deployErr error
		if pkgLayout.Pkg.IsInitConfig() {
			charts, deployErr = d.deployInitComponent(ctx, pkgLayout, component, opts)
		} else {
			charts, deployErr = d.deployComponent(ctx, pkgLayout, component, false, opts)
		}

		onDeploy := component.Actions.OnDeploy

		onFailure := func() {
			if err := actions.Run(ctx, onDeploy.Defaults, onDeploy.OnFailure, d.vc); err != nil {
				l.Debug("unable to run component failure action", "error", err.Error())
			}
		}

		if deployErr != nil {
			onFailure()
			deployedComponents[idx].Status = types.ComponentStatusFailed
			if d.isConnectedToCluster() {
				if _, err := d.c.RecordPackageDeployment(ctx, pkgLayout.Pkg, deployedComponents, packageGeneration); err != nil {
					l.Debug("unable to record package deployment", "component", component.Name, "error", err.Error())
				}
			}
			return nil, fmt.Errorf("unable to deploy component %q: %w", component.Name, deployErr)
		}

		// Update the package secret to indicate that we successfully deployed this component
		deployedComponents[idx].InstalledCharts = charts
		deployedComponents[idx].Status = types.ComponentStatusSucceeded
		if d.isConnectedToCluster() {
			if _, err := d.c.RecordPackageDeployment(ctx, pkgLayout.Pkg, deployedComponents, packageGeneration); err != nil {
				l.Debug("unable to record package deployment", "component", component.Name, "error", err.Error())
			}
		}

		if err := actions.Run(ctx, onDeploy.Defaults, onDeploy.OnSuccess, d.vc); err != nil {
			onFailure()
			return nil, fmt.Errorf("unable to run component success action: %w", err)
		}
	}

	return deployedComponents, nil
}

func (d *deployer) deployInitComponent(ctx context.Context, pkgLayout *layout.PackageLayout, component v1alpha1.ZarfComponent, opts DeployOpts) ([]types.InstalledChart, error) {
	l := logger.From(ctx)
	hasExternalRegistry := opts.InitStateOptions.RegistryInfo.Address != ""
	isSeedRegistry := component.Name == "zarf-seed-registry"
	isRegistry := component.Name == "zarf-registry"
	isInjector := component.Name == "zarf-injector"
	isAgent := component.Name == "zarf-agent"

	// Always init the state before the first component that requires the cluster (on most deployments, the zarf-seed-registry)
	if component.RequiresCluster() && d.s == nil {
		err := d.c.InitState(ctx, opts.InitStateOptions)
		if err != nil {
			return nil, fmt.Errorf("unable to initialize Zarf state: %w", err)
		}
	}

	if hasExternalRegistry && (isSeedRegistry || isInjector || isRegistry) {
		l.Info("skipping init package component since external registry information was provided", "component", component.Name)
		return nil, nil
	}

	if isRegistry {
		// If we are deploying the registry then mark the HPA as "modified" to set it to Min later
		d.hpaModified = true
	}

	// Before deploying the seed registry, start the injector
	if isSeedRegistry {
		err := d.c.StartInjection(ctx, pkgLayout.DirPath, pkgLayout.GetImageDir(), component.Images)
		if err != nil {
			return nil, err
		}
	}

	// Skip image checksum if component is agent.
	// Skip image push if component is seed registry.
	charts, err := d.deployComponent(ctx, pkgLayout, component, isAgent, opts)
	if err != nil {
		return nil, err
	}

	// Do cleanup for when we inject the seed registry during initialization
	if isSeedRegistry {
		if err := d.c.StopInjection(ctx); err != nil {
			return nil, fmt.Errorf("failed to delete injector resources: %w", err)
		}
	}

	return charts, nil
}

func (d *deployer) deployComponent(ctx context.Context, pkgLayout *layout.PackageLayout, component v1alpha1.ZarfComponent, noImgPush bool, opts DeployOpts) ([]types.InstalledChart, error) {
	l := logger.From(ctx)
	start := time.Now()

	l.Info("deploying component", "name", component.Name)

	hasImages := len(component.Images) > 0 && !noImgPush
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasFiles := len(component.Files) > 0

	onDeploy := component.Actions.OnDeploy

	if component.RequiresCluster() {
		// Setup the state in the config
		if d.s == nil {
			if err := d.setupState(ctx, d.c, pkgLayout.Pkg); err != nil {
				return nil, err
			}
		}

		// Disable the registry HPA scale down if we are deploying images and it is not already disabled
		if hasImages && !d.hpaModified && d.s.RegistryInfo.IsInternal() {
			if err := d.c.DisableRegHPAScaleDown(ctx); err != nil {
				l.Debug("unable to disable the registry HPA scale down", "error", err.Error())
			} else {
				d.hpaModified = true
			}
		}
	}

	err := populateComponentAndStateTemplates(ctx, component.Name, d.s, d.vc)
	if err != nil {
		return nil, err
	}

	if err = actions.Run(ctx, onDeploy.Defaults, onDeploy.Before, d.vc); err != nil {
		return nil, fmt.Errorf("unable to run component before action: %w", err)
	}

	if hasFiles {
		if err := processComponentFiles(ctx, pkgLayout, component, d.vc); err != nil {
			return nil, fmt.Errorf("unable to process the component files: %w", err)
		}
	}

	if hasImages {
		if err := pushImagesToRegistry(ctx, pkgLayout, d.s.RegistryInfo, false, opts.PlainHTTP, opts.OCIConcurrency, opts.Retries, opts.InsecureTLSSkipVerify); err != nil {
			return nil, fmt.Errorf("unable to push images to the registry: %w", err)
		}
	}

	if hasRepos {
		if err = pushReposToRepository(ctx, d.c, pkgLayout, d.s.GitServer, opts.Retries); err != nil {
			return nil, fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	g, gCtx := errgroup.WithContext(ctx)
	for idx, data := range component.DataInjections {
		tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmpDir)
		dataInjectionsPath, err := pkgLayout.GetComponentDir(tmpDir, component.Name, layout2.DataComponentDir)
		if err != nil {
			return nil, err
		}
		g.Go(func() error {
			return d.c.HandleDataInjection(gCtx, data, dataInjectionsPath, idx)
		})
	}

	charts := []types.InstalledChart{}
	if hasCharts {
		helmCharts, err := d.installCharts(ctx, pkgLayout, component, opts)
		if err != nil {
			return nil, err
		}
		charts = append(charts, helmCharts...)
	}

	if hasManifests {
		chartsFromManifests, err := d.installManifests(ctx, pkgLayout, component, opts)
		if err != nil {
			return nil, err
		}
		charts = append(charts, chartsFromManifests...)
	}

	if err = actions.Run(ctx, onDeploy.Defaults, onDeploy.After, d.vc); err != nil {
		return nil, fmt.Errorf("unable to run component after action: %w", err)
	}

	if len(component.HealthChecks) > 0 {
		healthCheckContext, cancel := context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
		l.Info("running health checks")
		if err = healthchecks.Run(healthCheckContext, d.c.Watcher, component.HealthChecks); err != nil {
			return nil, fmt.Errorf("health checks failed: %w", err)
		}
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}
	l.Debug("done deploying component", "name", component.Name, "duration", time.Since(start))
	return charts, nil
}

func (d *deployer) installCharts(ctx context.Context, pkgLayout *layout.PackageLayout, component v1alpha1.ZarfComponent, opts DeployOpts) ([]types.InstalledChart, error) {
	installedCharts := []types.InstalledChart{}

	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	chartDir, err := pkgLayout.GetComponentDir(tmpDir, component.Name, layout2.ChartsComponentDir)
	if err != nil {
		return nil, err
	}
	valuesDir, err := pkgLayout.GetComponentDir(tmpDir, component.Name, layout2.ValuesComponentDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to get values: %w", err)
	}

	for _, chart := range component.Charts {
		// Do not wait for the chart to be ready if data injections are present.
		if len(component.DataInjections) > 0 {
			chart.NoWait = true
		}

		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			valueFilePath := helm.StandardValuesName(valuesDir, chart, idx)
			if err := d.vc.ReplaceTextTemplate(valueFilePath); err != nil {
				return nil, err
			}
		}

		// Create a Helm values overrides map from set Zarf `variables` and DeployOpts library inputs
		// Values overrides are to be applied in order of Helm Chart Defaults -> Zarf `valuesFiles` -> Zarf `variables` -> DeployOpts overrides
		valuesOverrides, err := generateValuesOverrides(chart, component.Name, d.vc, opts.ValuesOverridesMap)
		if err != nil {
			return nil, err
		}

		helmOpts := helm.InstallUpgradeOpts{
			AdoptExistingResources: opts.AdoptExistingResources,
			VariableConfig:         d.vc,
			State:                  d.s,
			Cluster:                d.c,
			AirgapMode:             !pkgLayout.Pkg.Metadata.YOLO,
			Timeout:                opts.Timeout,
			Retries:                opts.Retries,
		}
		helmChart, values, err := helm.LoadChartData(chart, chartDir, valuesDir, valuesOverrides)
		if err != nil {
			return nil, fmt.Errorf("failed to load chart data: %w", err)
		}

		connectStrings, installedChartName, err := helm.InstallOrUpgradeChart(ctx, chart, helmChart, values, helmOpts)
		if err != nil {
			return nil, err
		}
		installedCharts = append(installedCharts, types.InstalledChart{Namespace: chart.Namespace, ChartName: installedChartName, ConnectStrings: connectStrings})
	}

	return installedCharts, nil
}

func (d *deployer) installManifests(ctx context.Context, pkgLayout *layout.PackageLayout, component v1alpha1.ZarfComponent, opts DeployOpts) ([]types.InstalledChart, error) {
	tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)
	manifestDir, err := pkgLayout.GetComponentDir(tmpDir, component.Name, layout2.ManifestsComponentDir)
	if err != nil {
		return nil, err
	}

	installedCharts := []types.InstalledChart{}
	for _, manifest := range component.Manifests {
		for idx := range manifest.Files {
			if helpers.InvalidPath(filepath.Join(manifestDir, manifest.Files[idx])) {
				// The path is likely invalid because of how we compose OCI components, add an index suffix to the filename
				manifest.Files[idx] = fmt.Sprintf("%s-%d.yaml", manifest.Name, idx)
				if helpers.InvalidPath(filepath.Join(manifestDir, manifest.Files[idx])) {
					return nil, fmt.Errorf("unable to find manifest file %s", manifest.Files[idx])
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

		// Create a helmChart and helm cfg from a given Zarf Manifest.
		chart, helmChart, err := helm.ChartFromZarfManifest(manifest, manifestDir, pkgLayout.Pkg.Metadata.Name, component.Name)
		if err != nil {
			return nil, err
		}
		helmOpts := helm.InstallUpgradeOpts{
			AdoptExistingResources: opts.AdoptExistingResources,
			VariableConfig:         d.vc,
			State:                  d.s,
			Cluster:                d.c,
			AirgapMode:             !pkgLayout.Pkg.Metadata.YOLO,
			Timeout:                opts.Timeout,
			Retries:                opts.Retries,
		}

		// Install the chart.
		connectStrings, installedChartName, err := helm.InstallOrUpgradeChart(ctx, chart, helmChart, nil, helmOpts)
		if err != nil {
			return nil, err
		}
		installedCharts = append(installedCharts, types.InstalledChart{Namespace: manifest.Namespace, ChartName: installedChartName, ConnectStrings: connectStrings})
	}

	return installedCharts, nil
}

// setupState fetches the current State from the k8s cluster and sets the deployer to use it
func (d *deployer) setupState(ctx context.Context, c *cluster.Cluster, pkg v1alpha1.ZarfPackage) error {
	l := logger.From(ctx)
	// If we are touching K8s, make sure we can talk to it once per deployment
	l.Debug("loading the Zarf State from the Kubernetes cluster")

	s, err := c.LoadState(ctx)
	// We ignore the error if in YOLO mode because Zarf should not be initiated.
	if err != nil && !pkg.Metadata.YOLO {
		return err
	}
	// Only ignore state load error in yolo mode when secret could not be found.
	if err != nil && !kerrors.IsNotFound(err) && pkg.Metadata.YOLO {
		return err
	}
	if s == nil && pkg.Metadata.YOLO {
		s = &state.State{}
		// YOLO mode, so minimal state needed
		s.Distro = "YOLO"

		l.Info("creating the Zarf namespace")
		zarfNamespace := cluster.NewZarfManagedApplyNamespace(state.ZarfNamespaceName)
		_, err = c.Clientset.CoreV1().Namespaces().Apply(ctx, zarfNamespace, metav1.ApplyOptions{Force: true, FieldManager: cluster.FieldManagerName})
		if err != nil {
			return fmt.Errorf("unable to apply the Zarf namespace: %w", err)
		}
	}
	if s == nil {
		return errors.New("cluster state should not be nil")
	}
	if pkg.Metadata.YOLO && s.Distro != "YOLO" {
		l.Warn("This package is in YOLO mode, but the cluster was already initialized with 'zarf init'. " +
			"This may cause issues if the package does not exclude any charts or manifests from the Zarf Agent using " +
			"the pod or namespace label `zarf.dev/agent: ignore'.")
	}

	d.s = s
	return nil
}

func (d *deployer) connectToCluster(ctx context.Context, pkg v1alpha1.ZarfPackage) error {
	// If we are already connected to the cluster then return
	c, err := cluster.NewWithWait(ctx)
	if err != nil {
		return err
	}

	if err := attemptClusterChecks(ctx, c, pkg); err != nil {
		return err
	}

	d.c = c

	return nil
}

func attemptClusterChecks(ctx context.Context, c *cluster.Cluster, pkg v1alpha1.ZarfPackage) error {
	// Check the clusters architecture matches the package spec
	if err := validatePackageArchitecture(ctx, c, pkg); err != nil {
		if errors.Is(err, lang.ErrUnableToCheckArch) {
			logger.From(ctx).Warn("unable to validate package architecture", "error", err)
		} else {
			return err
		}
	}

	// Check for any breaking changes between the initialized Zarf version and this CLI
	if existingInitPackage, _ := c.GetDeployedPackage(ctx, "init"); existingInitPackage != nil {
		// Use the build version instead of the metadata since this will support older Zarf versions
		err := deprecated.PrintBreakingChanges(os.Stderr, existingInitPackage.Data.Build.Version, config.CLIVersion)
		if err != nil {
			return err
		}
	}

	s, err := c.LoadState(ctx)
	if err != nil {
		// don't return the err here as state may not yet be setup
		return nil
	}
	return pki.CheckForExpiredCert(ctx, s.AgentTLS)
}

func validatePackageArchitecture(ctx context.Context, c *cluster.Cluster, pkg v1alpha1.ZarfPackage) error {
	// Ignore this check if we don't have a cluster connection, or the package contains no images
	if !pkg.HasImages() || c == nil {
		return nil
	}

	// Get node architectures
	nodeList, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return lang.ErrUnableToCheckArch
	}
	if len(nodeList.Items) == 0 {
		return lang.ErrUnableToCheckArch
	}
	archMap := map[string]bool{}
	for _, node := range nodeList.Items {
		archMap[node.Status.NodeInfo.Architecture] = true
	}
	architectures := []string{}
	for arch := range archMap {
		architectures = append(architectures, arch)
	}

	// Check if the package architecture and the cluster architecture are the same.
	if !slices.Contains(architectures, pkg.Metadata.Architecture) {
		return fmt.Errorf(lang.CmdPackageDeployValidateArchitectureErr, pkg.Metadata.Architecture, strings.Join(architectures, ", "))
	}

	return nil
}

func populateComponentAndStateTemplates(ctx context.Context, componentName string, s *state.State, variableConfig *variables.VariableConfig) error {
	applicationTemplates, err := template.GetZarfTemplates(ctx, componentName, s)
	if err != nil {
		return err
	}
	variableConfig.SetApplicationTemplates(applicationTemplates)
	return nil
}

func processComponentFiles(ctx context.Context, pkgLayout *layout.PackageLayout, component v1alpha1.ZarfComponent, variableConfig *variables.VariableConfig) error {
	l := logger.From(ctx)
	start := time.Now()
	l.Info("copying files", "count", len(component.Files))

	tmpdir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return err
	}

	filesDir, err := pkgLayout.GetComponentDir(tmpdir, component.Name, layout.FilesComponentDir)
	if err != nil {
		return err
	}

	for fileIdx, file := range component.Files {
		l.Info("loading file", "name", file.Target)

		fileLocation := filepath.Join(filesDir, strconv.Itoa(fileIdx), filepath.Base(file.Target))
		if helpers.InvalidPath(fileLocation) {
			fileLocation = filepath.Join(filesDir, strconv.Itoa(fileIdx))
		}

		// If a shasum is specified check it again on deployment as well
		if file.Shasum != "" {
			l.Debug("Validating SHASUM", "file", file.Target)
			if err := helpers.SHAsMatch(fileLocation, file.Shasum); err != nil {
				return err
			}
		}

		// Replace temp target directory and home directory
		target, err := config.GetAbsHomePath(strings.Replace(file.Target, "###ZARF_TEMP###", pkgLayout.DirPath, 1))
		if err != nil {
			return err
		}
		file.Target = target

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
				return err
			}

			// If the file is a text file, template it
			if isText {
				l.Debug("template file", "name", file.Target)
				if err := variableConfig.ReplaceTextTemplate(subFile); err != nil {
					return fmt.Errorf("unable to template file %s: %w", subFile, err)
				}
			}
		}

		// Copy the file to the destination
		l.Debug("saving file", "name", file.Target)
		err = helpers.CreatePathAndCopy(fileLocation, file.Target)
		if err != nil {
			return fmt.Errorf("unable to copy file %s to %s: %w", fileLocation, file.Target, err)
		}

		// Loop over all symlinks and create them
		for _, link := range file.Symlinks {
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

	l.Debug("done copying files", "duration", time.Since(start))

	return nil
}

func generateValuesOverrides(chart v1alpha1.ZarfChart, componentName string, variableConfig *variables.VariableConfig, valuesOverridesMap map[string]map[string]map[string]interface{}) (map[string]any, error) {
	valuesOverrides := make(map[string]any)
	chartOverrides := make(map[string]any)

	for _, variable := range chart.Variables {
		if setVar, ok := variableConfig.GetSetVariable(variable.Name); ok && setVar != nil {
			// Use the variable's path as a key to ensure unique entries for variables with the same name but different paths.
			if err := helpers.MergePathAndValueIntoMap(chartOverrides, variable.Path, setVar.Value); err != nil {
				return nil, fmt.Errorf("unable to merge path and value into map: %w", err)
			}
		}
	}

	// Apply any direct overrides specified in the deployment options for this component and chart
	if componentOverrides, ok := valuesOverridesMap[componentName]; ok {
		if chartSpecificOverrides, ok := componentOverrides[chart.Name]; ok {
			valuesOverrides = chartSpecificOverrides
		}
	}

	// Merge chartOverrides into valuesOverrides to ensure all overrides are applied.
	// This corrects the logic to ensure that chartOverrides and valuesOverrides are merged correctly.
	return helpers.MergeMapRecursive(chartOverrides, valuesOverrides), nil
}
