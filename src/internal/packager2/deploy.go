package packager2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	actions2 "github.com/zarf-dev/zarf/src/internal/packager2/actions"
	"github.com/zarf-dev/zarf/src/internal/packager2/helm"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/deprecated"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// localClusterServiceRegex is used to match the local cluster service format:
	localClusterServiceRegex = regexp.MustCompile(`^(?P<name>[^\.]+)\.(?P<namespace>[^\.]+)\.svc\.cluster\.local$`)
)

type DeployOptions struct {
	Source             string
	OptionalComponents string
	SetVariables       map[string]string
}

func Deploy(ctx context.Context, opt DeployOptions) ([]types.DeployedComponent, error) {
	l := logger.From(ctx)
	start := time.Now()

	isInteractive := !config.CommonOptions.Confirm
	loadOpts := LoadOptions{
		Source: opt.Source,
		Filter: filters.Empty(),
	}
	if isInteractive {
		loadOpts.Filter = filters.Combine(
			filters.ByLocalOS(runtime.GOOS),
			filters.ForDeploy(opt.OptionalComponents, isInteractive),
		)
	}
	pkgLayout, err := LoadPackage(ctx, loadOpts)
	if err != nil {
		return nil, err
	}

	warnings := []string{}
	validateWarnings, err := validateLastNonBreakingVersion(config.CLIVersion, pkgLayout.Pkg.Build.LastNonBreakingVersion)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, validateWarnings...)

	if isInteractive {
		var err error
		pkgLayout.Pkg.Components, err = loadOpts.Filter.Apply(pkgLayout.Pkg)
		if err != nil {
			return nil, err
		}
	}

	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkgLayout.Pkg.Constants)
	variableConfig.PopulateVariables(pkgLayout.Pkg.Variables, opt.SetVariables)

	// p.hpaModified = false
	// Reset registry HPA scale down whether an error occurs or not
	// defer p.resetRegistryHPA(ctx)

	var deployedComponents []types.DeployedComponent
	var c *cluster.Cluster
	var state *types.ZarfState

	// Process all the components we are deploying
	for _, component := range pkgLayout.Pkg.Components {
		// Connect to cluster if a component requires it.
		if component.RequiresCluster() && c == nil {
			timeout := cluster.DefaultTimeout
			if pkgLayout.Pkg.IsInitConfig() {
				timeout = 5 * time.Minute
			}
			connectCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			var err error
			c, err = cluster.NewClusterWithWait(connectCtx)
			if err != nil {
				return nil, err
			}

			if state == nil {
				state, err = setupState(ctx, c, pkgLayout.Pkg)
				if err != nil {
					return nil, err
				}
			}
		}

		deployedComponent := types.DeployedComponent{
			Name: component.Name,
		}

		// Ensure we don't overwrite any installedCharts data when updating the package secret
		if c != nil {
			installedCharts, err := c.GetInstalledChartsForComponent(ctx, pkgLayout.Pkg.Metadata.Name, component)
			if err != nil {
				message.Debugf("Unable to fetch installed Helm charts for component '%s': %s", component.Name, err.Error())
				l.Debug("unable to fetch installed Helm charts", "component", component.Name, "error", err.Error())
			}
			deployedComponent.InstalledCharts = installedCharts
		}

		deployedComponents = append(deployedComponents, deployedComponent)
		idx := len(deployedComponents) - 1

		// Deploy the component
		var charts []types.InstalledChart
		var deployErr error
		if pkgLayout.Pkg.IsInitConfig() {
			// charts, deployErr = deployInitComponent(ctx, c, component)
		} else {
			charts, deployErr = deployComponent(ctx, c, pkgLayout, component, variableConfig, state)
		}

		onDeploy := component.Actions.OnDeploy

		onFailure := func() {
			if err := actions2.Run(ctx, pkgLayout.GetBasePath(), onDeploy.Defaults, onDeploy.OnFailure, variableConfig); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
				l.Debug("unable to run component failure action", "error", err.Error())
			}
		}

		if deployErr != nil {
			onFailure()
			if c != nil {
				if _, err := c.RecordPackageDeployment(ctx, pkgLayout.Pkg, deployedComponents); err != nil {
					message.Debugf("Unable to record package deployment for component %q: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
					l.Debug("unable to record package deployment", "component", component.Name, "error", err.Error())
				}
			}
			return nil, fmt.Errorf("unable to deploy component %q: %w", component.Name, deployErr)
		}

		// Update the package secret to indicate that we successfully deployed this component
		deployedComponents[idx].InstalledCharts = charts
		if c != nil {
			if _, err := c.RecordPackageDeployment(ctx, pkgLayout.Pkg, deployedComponents); err != nil {
				message.Debugf("Unable to record package deployment for component %q: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
				l.Debug("unable to record package deployment", "component", component.Name, "error", err.Error())
			}
		}

		if err := actions2.Run(ctx, pkgLayout.GetBasePath(), onDeploy.Defaults, onDeploy.OnSuccess, variableConfig); err != nil {
			onFailure()
			return nil, fmt.Errorf("unable to run component success action: %w", err)
		}
	}

	if len(deployedComponents) == 0 {
		message.Warn("No components were selected for deployment.  Inspect the package to view the available components and select components interactively or by name with \"--components\"")
		l.Warn("no components were selected for deployment. Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf deployment complete")
	l.Debug("Zarf deployment complete", "duration", time.Since(start))

	return deployedComponents, nil
}

func deployComponent(ctx context.Context, c *cluster.Cluster, pkgLayout *layout2.PackageLayout, component v1alpha1.ZarfComponent, variableConfig *variables.VariableConfig, state *types.ZarfState) ([]types.InstalledChart, error) {
	retries := 3
	noImgChecksum := false
	noImgPush := false

	l := logger.From(ctx)
	start := time.Now()

	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))
	l.Info("deploying component", "name", component.Name)

	onDeploy := component.Actions.OnDeploy

	// if component.RequiresCluster() {
	// Disable the registry HPA scale down if we are deploying images and it is not already disabled
	// if hasImages && !p.hpaModified && p.state.RegistryInfo.IsInternal() {
	// 	if err := p.cluster.DisableRegHPAScaleDown(ctx); err != nil {
	// 		message.Debugf("unable to disable the registry HPA scale down: %s", err.Error())
	// 		l.Debug("unable to disable the registry HPA scale down", "error", err.Error())
	// 	} else {
	// 		p.hpaModified = true
	// 	}
	// }
	// }

	applicationTemplates, err := template.GetZarfTemplates(ctx, component.Name, state)
	if err != nil {
		return nil, err
	}
	variableConfig.SetApplicationTemplates(applicationTemplates)

	if err = actions2.Run(ctx, pkgLayout.GetBasePath(), onDeploy.Defaults, onDeploy.Before, variableConfig); err != nil {
		return nil, fmt.Errorf("unable to run component before action: %w", err)
	}

	// if len(component.Files) > 0 {
	// 	if err := processComponentFiles(ctx, pkgLayout, component, variableConfig); err != nil {
	// 		return nil, fmt.Errorf("unable to process the component files: %w", err)
	// 	}
	// }
	if len(component.Images) > 0 && !noImgPush {
		if err := pushImagesToRegistry(ctx, c, pkgLayout, filters.Empty(), types.RegistryInfo{}, noImgChecksum, retries); err != nil {
			return nil, fmt.Errorf("unable to push images to the registry: %w", err)
		}
	}
	if len(component.Repos) > 0 {
		if err = pushReposToRepository(ctx, c, pkgLayout, filters.Empty(), types.GitServerInfo{}, retries); err != nil {
			return nil, fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	g, gCtx := errgroup.WithContext(ctx)
	for idx, data := range component.DataInjections {
		g.Go(func() error {
			tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmp)
			dataPath, err := pkgLayout.GetComponentDir(tmp, component.Name, layout2.DataComponentDir)
			if err != nil {
				return err
			}
			return c.InjectData(gCtx, data, dataPath, idx)
		})
	}

	charts := []types.InstalledChart{}
	if len(component.Charts) > 0 || len(component.Manifests) > 0 {
		charts, err = installChartAndManifests(ctx, c, pkgLayout, component, variableConfig, state)
		if err != nil {
			return nil, err
		}
	}

	if err = actions2.Run(ctx, pkgLayout.GetBasePath(), onDeploy.Defaults, onDeploy.After, variableConfig); err != nil {
		return nil, fmt.Errorf("unable to run component after action: %w", err)
	}

	if len(component.HealthChecks) > 0 {
		// TODO: Make configurable
		deployTimeout := 5 * time.Minute
		healthCheckContext, cancel := context.WithTimeout(ctx, deployTimeout)
		defer cancel()
		spinner := message.NewProgressSpinner("Running health checks")
		l.Info("running health checks")
		defer spinner.Stop()
		if err = healthchecks.Run(healthCheckContext, c.Watcher, component.HealthChecks); err != nil {
			return nil, fmt.Errorf("health checks failed: %w", err)
		}
		spinner.Success()
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}
	l.Debug("done deploying component", "name", component.Name, "duration", time.Since(start))
	return charts, nil
}

// attemptClusterChecks attempts to connect to the cluster and check for useful metadata and config mismatches.
// NOTE: attemptClusterChecks should only return an error if there is a problem significant enough to halt a deployment, otherwise it should return nil and print a warning message.
func attemptClusterChecks(ctx context.Context, c *cluster.Cluster, pkg v1alpha1.ZarfPackage) error {
	// Check the clusters architecture matches the package spec
	if err := validatePackageArchitecture(ctx, c, pkg); err != nil {
		if errors.Is(err, lang.ErrUnableToCheckArch) {
			message.Warnf("Unable to validate package architecture: %s", err.Error())
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

	return nil
}

// validatePackageArchitecture validates that the package architecture matches the target cluster architecture.
func validatePackageArchitecture(ctx context.Context, c *cluster.Cluster, pkg v1alpha1.ZarfPackage) error {
	// Ignore this check if we don't have a cluster connection, or the package contains no images
	if c == nil || !pkg.HasImages() {
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

// validateLastNonBreakingVersion validates the Zarf CLI version against a package's LastNonBreakingVersion.
func validateLastNonBreakingVersion(cliVersion, lastNonBreakingVersion string) ([]string, error) {
	if lastNonBreakingVersion == "" {
		return nil, nil
	}
	lastNonBreakingSemVer, err := semver.NewVersion(lastNonBreakingVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to parse last non breaking version %s from Zarf package build data: %w", lastNonBreakingVersion, err)
	}
	cliSemVer, err := semver.NewVersion(cliVersion)
	if err != nil {
		return []string{fmt.Sprintf(lang.CmdPackageDeployInvalidCLIVersionWarn, cliVersion)}, nil
	}
	if cliSemVer.LessThan(lastNonBreakingSemVer) {
		warning := fmt.Sprintf(
			lang.CmdPackageDeployValidateLastNonBreakingVersionWarn,
			cliVersion,
			lastNonBreakingVersion,
			lastNonBreakingVersion,
		)
		return []string{warning}, nil
	}
	return nil, nil
}

func installChartAndManifests(ctx context.Context, c *cluster.Cluster, pkgLayout *layout2.PackageLayout, component v1alpha1.ZarfComponent, variableConfig *variables.VariableConfig, state *types.ZarfState) ([]types.InstalledChart, error) {
	timeout := 10 * time.Second
	retries := 3
	adoptExistingResources := true

	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	valuesDir, err := pkgLayout.GetComponentDir(tmp, component.Name, layout2.ValuesComponentDir)
	if err != nil {
		return nil, err
	}
	chartDir, err := pkgLayout.GetComponentDir(tmp, component.Name, layout2.ChartsComponentDir)
	if err != nil {
		return nil, err
	}
	manifestsDir, err := pkgLayout.GetComponentDir(tmp, component.Name, layout2.ManifestsComponentDir)
	if err != nil {
		return nil, err
	}

	installedCharts := []types.InstalledChart{}
	for _, chart := range component.Charts {
		// Do not wait for the chart to be ready if data injections are present.
		if len(component.DataInjections) > 0 {
			chart.NoWait = true
		}

		// zarf magic for the value file
		for idx := range chart.ValuesFiles {
			valueFilePath := helm.StandardValuesName(valuesDir, chart, idx)
			if err := variableConfig.ReplaceTextTemplate(valueFilePath); err != nil {
				return nil, err
			}
		}

		// Create a Helm values overrides map from set Zarf `variables` and DeployOpts library inputs
		// Values overrides are to be applied in order of Helm Chart Defaults -> Zarf `valuesFiles` -> Zarf `variables` -> DeployOpts overrides
		valuesOverrides, err := generateValuesOverrides(chart, variableConfig, component.Name)
		if err != nil {
			return nil, err
		}

		helmCfg := helm.New(
			chart,
			chartDir,
			valuesDir,
			helm.WithDeployInfo(
				variableConfig,
				state,
				c,
				valuesOverrides,
				adoptExistingResources,
				pkgLayout.Pkg.Metadata.YOLO,
				timeout,
				retries,
			),
		)
		connectStrings, installedChartName, err := helmCfg.InstallOrUpgradeChart(ctx)
		if err != nil {
			return nil, err
		}
		installedCharts = append(installedCharts, types.InstalledChart{Namespace: chart.Namespace, ChartName: installedChartName, ConnectStrings: connectStrings})
	}

	for _, manifest := range component.Manifests {
		for idx := range manifest.Files {
			if helpers.InvalidPath(filepath.Join(manifestsDir, manifest.Files[idx])) {
				// The path is likely invalid because of how we compose OCI components, add an index suffix to the filename
				manifest.Files[idx] = fmt.Sprintf("%s-%d.yaml", manifest.Name, idx)
				if helpers.InvalidPath(filepath.Join(manifestsDir, manifest.Files[idx])) {
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

		// Create a chart and helm cfg from a given Zarf Manifest.
		helmCfg, err := helm.NewFromZarfManifest(
			manifest,
			manifestsDir,
			pkgLayout.Pkg.Metadata.Name,
			component.Name,
			helm.WithDeployInfo(
				variableConfig,
				state,
				c,
				nil,
				adoptExistingResources,
				pkgLayout.Pkg.Metadata.YOLO,
				timeout,
				retries,
			),
		)
		if err != nil {
			return nil, err
		}

		// Install the chart.
		connectStrings, installedChartName, err := helmCfg.InstallOrUpgradeChart(ctx)
		if err != nil {
			return nil, err
		}
		installedCharts = append(installedCharts, types.InstalledChart{Namespace: manifest.Namespace, ChartName: installedChartName, ConnectStrings: connectStrings})
	}

	return installedCharts, nil
}

func generateValuesOverrides(chart v1alpha1.ZarfChart, variableConfig *variables.VariableConfig, componentName string) (map[string]any, error) {
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
	// if componentOverrides, ok := p.cfg.DeployOpts.ValuesOverridesMap[componentName]; ok {
	// 	if chartSpecificOverrides, ok := componentOverrides[chart.Name]; ok {
	// 		valuesOverrides = chartSpecificOverrides
	// 	}
	// }

	// Merge chartOverrides into valuesOverrides to ensure all overrides are applied.
	// This corrects the logic to ensure that chartOverrides and valuesOverrides are merged correctly.
	return helpers.MergeMapRecursive(chartOverrides, valuesOverrides), nil
}

// Move files onto the host of the machine performing the deployment.
// func processComponentFiles(ctx context.Context, pkgLayout *layout2.PackageLayout, component v1alpha1.ZarfComponent, variableConfig *variables.VariableConfig) error {
// 	l := logger.From(ctx)
// 	spinner := message.NewProgressSpinner("Copying %d files", len(component.Files))
// 	start := time.Now()
// 	l.Info("copying files", "count", len(component.Files))
// 	defer spinner.Stop()

// 	for fileIdx, file := range component.Files {
// 		spinner.Updatef("Loading %s", file.Target)
// 		l.Info("loading file", "name", file.Target)

// 		fileLocation := filepath.Join(pkgLocation, strconv.Itoa(fileIdx), filepath.Base(file.Target))
// 		if helpers.InvalidPath(fileLocation) {
// 			fileLocation = filepath.Join(pkgLocation, strconv.Itoa(fileIdx))
// 		}

// 		// If a shasum is specified check it again on deployment as well
// 		if file.Shasum != "" {
// 			spinner.Updatef("Validating SHASUM for %s", file.Target)
// 			l.Debug("Validating SHASUM", "file", file.Target)
// 			if err := helpers.SHAsMatch(fileLocation, file.Shasum); err != nil {
// 				return err
// 			}
// 		}

// 		// Replace temp target directory and home directory
// 		var err error
// 		// target, err := config.GetAbsHomePath(strings.Replace(file.Target, "###ZARF_TEMP###", p.layout.Base, 1))
// 		// if err != nil {
// 		// 	return err
// 		// }
// 		// file.Target = target

// 		fileList := []string{}
// 		if helpers.IsDir(fileLocation) {
// 			files, _ := helpers.RecursiveFileList(fileLocation, nil, false)
// 			fileList = append(fileList, files...)
// 		} else {
// 			fileList = append(fileList, fileLocation)
// 		}

// 		for _, subFile := range fileList {
// 			// Check if the file looks like a text file
// 			isText, err := helpers.IsTextFile(subFile)
// 			if err != nil {
// 				return err
// 			}

// 			// If the file is a text file, template it
// 			if isText {
// 				spinner.Updatef("Templating %s", file.Target)
// 				l.Debug("template file", "name", file.Target)
// 				if err := variableConfig.ReplaceTextTemplate(subFile); err != nil {
// 					return fmt.Errorf("unable to template file %s: %w", subFile, err)
// 				}
// 			}
// 		}

// 		// Copy the file to the destination
// 		spinner.Updatef("Saving %s", file.Target)
// 		l.Debug("saving file", "name", file.Target)
// 		err = helpers.CreatePathAndCopy(fileLocation, file.Target)
// 		if err != nil {
// 			return fmt.Errorf("unable to copy file %s to %s: %w", fileLocation, file.Target, err)
// 		}

// 		// Loop over all symlinks and create them
// 		for _, link := range file.Symlinks {
// 			spinner.Updatef("Adding symlink %s->%s", link, file.Target)
// 			// Try to remove the filepath if it exists
// 			_ = os.RemoveAll(link)
// 			// Make sure the parent directory exists
// 			_ = helpers.CreateParentDirectory(link)
// 			// Create the symlink
// 			err := os.Symlink(file.Target, link)
// 			if err != nil {
// 				return fmt.Errorf("unable to create symlink %s->%s: %w", link, file.Target, err)
// 			}
// 		}

// 		// Cleanup now to reduce disk pressure
// 		_ = os.RemoveAll(fileLocation)
// 	}

// 	spinner.Success()
// 	l.Debug("done copying files", "duration", time.Since(start))

// 	return nil
// }

func setupState(ctx context.Context, c *cluster.Cluster, pkg v1alpha1.ZarfPackage) (*types.ZarfState, error) {
	l := logger.From(ctx)
	// If we are touching K8s, make sure we can talk to it once per deployment
	spinner := message.NewProgressSpinner("Loading the Zarf State from the Kubernetes cluster")
	defer spinner.Stop()
	l.Debug("loading the Zarf State from the Kubernetes cluster")

	state, err := c.LoadZarfState(ctx)
	// We ignore the error if in YOLO mode because Zarf should not be initiated.
	if err != nil && !pkg.Metadata.YOLO {
		return nil, err
	}
	// Only ignore state load error in yolo mode when secret could not be found.
	if err != nil && !kerrors.IsNotFound(err) && pkg.Metadata.YOLO {
		return nil, err
	}
	if state == nil && pkg.Metadata.YOLO {
		state = &types.ZarfState{}
		// YOLO mode, so minimal state needed
		state.Distro = "YOLO"

		spinner.Updatef("Creating the Zarf namespace")
		l.Info("creating the Zarf namespace")
		zarfNamespace := cluster.NewZarfManagedApplyNamespace(cluster.ZarfNamespaceName)
		_, err = c.Clientset.CoreV1().Namespaces().Apply(ctx, zarfNamespace, metav1.ApplyOptions{Force: true, FieldManager: cluster.FieldManagerName})
		if err != nil {
			return nil, fmt.Errorf("unable to apply the Zarf namespace: %w", err)
		}
	}

	if pkg.Metadata.YOLO && state.Distro != "YOLO" {
		message.Warn("This package is in YOLO mode, but the cluster was already initialized with 'zarf init'. " +
			"This may cause issues if the package does not exclude any charts or manifests from the Zarf Agent using " +
			"the pod or namespace label `zarf.dev/agent: ignore'.")
		l.Warn("This package is in YOLO mode, but the cluster was already initialized with 'zarf init'. " +
			"This may cause issues if the package does not exclude any charts or manifests from the Zarf Agent using " +
			"the pod or namespace label `zarf.dev/agent: ignore'.")
	}

	spinner.Success()
	return state, nil
}
