package packager2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/deprecated"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/types"
)

var (
	// localClusterServiceRegex is used to match the local cluster service format:
	localClusterServiceRegex = regexp.MustCompile(`^(?P<name>[^\.]+)\.(?P<namespace>[^\.]+)\.svc\.cluster\.local$`)
)

type DeployOptions struct {
	OptionalComponents string
}

func Deploy(ctx context.Context, opt DeployOptions) error {
	l := logger.From(ctx)
	start := time.Now()
	isInteractive := !config.CommonOptions.Confirm

	deployFilter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ForDeploy(opt.OptionalComponents, isInteractive),
	)

	var pkgLayout layout2.PackageLayout

	warnings := []string{}
	// if isInteractive {
	// 	filter := filters.Empty()
	// 	pkg, loadWarnings, err := p.source.LoadPackage(ctx, p.layout, filter, true)
	// 	if err != nil {
	// 		return fmt.Errorf("unable to load the package: %w", err)
	// 	}
	// 	p.cfg.Pkg = pkg
	// 	warnings = append(warnings, loadWarnings...)
	// } else {
	// 	pkg, loadWarnings, err := p.source.LoadPackage(ctx, p.layout, deployFilter, true)
	// 	if err != nil {
	// 		return fmt.Errorf("unable to load the package: %w", err)
	// 	}
	// 	p.cfg.Pkg = pkg
	// 	warnings = append(warnings, loadWarnings...)
	// 	if err := p.populatePackageVariableConfig(); err != nil {
	// 		return fmt.Errorf("unable to set the active variables: %w", err)
	// 	}
	// }

	// validateWarnings, err := validateLastNonBreakingVersion(config.CLIVersion, p.cfg.Pkg.Build.LastNonBreakingVersion)
	// if err != nil {
	// 	return err
	// }
	// warnings = append(warnings, validateWarnings...)

	// sbomViewFiles, sbomWarnings, err := p.layout.SBOMs.StageSBOMViewFiles()
	// if err != nil {
	// 	return err
	// }
	// warnings = append(warnings, sbomWarnings...)

	// Confirm the overall package deployment
	// if !p.confirmAction(ctx, config.ZarfDeployStage, warnings, sbomViewFiles) {
	// 	return fmt.Errorf("deployment cancelled")
	// }

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

	// p.hpaModified = false
	// // Reset registry HPA scale down whether an error occurs or not
	// defer p.resetRegistryHPA(ctx)

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := .deployComponents(ctx)
	if err != nil {
		return err
	}
	if len(deployedComponents) == 0 {
		message.Warn("No components were selected for deployment.  Inspect the package to view the available components and select components interactively or by name with \"--components\"")
		l.Warn("no components were selected for deployment. Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment
	message.Successf("Zarf deployment complete")
	l.Debug("Zarf deployment complete", "duration", time.Since(start))

	// err = p.printTablesForDeployment(ctx, deployedComponents)
	// if err != nil {
	// 	return err
	// }

	return nil
}

// deployComponents loops through a list of ZarfComponents and deploys them.
func deployComponents(ctx context.Context) ([]types.DeployedComponent, error) {
	l := logger.From(ctx)
	deployedComponents := []types.DeployedComponent{}

	// Process all the components we are deploying
	for _, component := range p.cfg.Pkg.Components {
		// Connect to cluster if a component requires it.
		if component.RequiresCluster() {
			timeout := cluster.DefaultTimeout
			if p.cfg.Pkg.IsInitConfig() {
				timeout = 5 * time.Minute
			}
			connectCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if err := p.connectToCluster(connectCtx); err != nil {
				return nil, fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
			}
		}

		deployedComponent := types.DeployedComponent{
			Name: component.Name,
		}

		// Ensure we don't overwrite any installedCharts data when updating the package secret
		if p.isConnectedToCluster() {
			installedCharts, err := p.cluster.GetInstalledChartsForComponent(ctx, p.cfg.Pkg.Metadata.Name, component)
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
		if p.cfg.Pkg.IsInitConfig() {
			charts, deployErr = p.deployInitComponent(ctx, component)
		} else {
			charts, deployErr = p.deployComponent(ctx, component, false, false)
		}

		onDeploy := component.Actions.OnDeploy

		onFailure := func() {
			if err := actions.Run(ctx, onDeploy.Defaults, onDeploy.OnFailure, p.variableConfig); err != nil {
				message.Debugf("unable to run component failure action: %s", err.Error())
				l.Debug("unable to run component failure action", "error", err.Error())
			}
		}

		if deployErr != nil {
			onFailure()

			if p.isConnectedToCluster() {
				if _, err := p.cluster.RecordPackageDeployment(ctx, p.cfg.Pkg, deployedComponents); err != nil {
					message.Debugf("Unable to record package deployment for component %q: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
					l.Debug("unable to record package deployment", "component", component.Name, "error", err.Error())
				}
			}
			return nil, fmt.Errorf("unable to deploy component %q: %w", component.Name, deployErr)
		}

		// Update the package secret to indicate that we successfully deployed this component
		deployedComponents[idx].InstalledCharts = charts
		if p.isConnectedToCluster() {
			if _, err := p.cluster.RecordPackageDeployment(ctx, p.cfg.Pkg, deployedComponents); err != nil {
				message.Debugf("Unable to record package deployment for component %q: this will affect features like `zarf package remove`: %s", component.Name, err.Error())
				l.Debug("unable to record package deployment", "component", component.Name, "error", err.Error())
			}
		}

		if err := actions.Run(ctx, onDeploy.Defaults, onDeploy.OnSuccess, p.variableConfig); err != nil {
			onFailure()
			return nil, fmt.Errorf("unable to run component success action: %w", err)
		}
	}

	return deployedComponents, nil
}

func deployInitComponent(ctx context.Context, component v1alpha1.ZarfComponent) ([]types.InstalledChart, error) {
	l := logger.From(ctx)
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
		err := p.cluster.InitZarfState(ctx, p.cfg.InitOpts)
		if err != nil {
			return nil, fmt.Errorf("unable to initialize Zarf state: %w", err)
		}
	}

	if hasExternalRegistry && (isSeedRegistry || isInjector || isRegistry) {
		message.Notef("Not deploying the component (%s) since external registry information was provided during `zarf init`", component.Name)
		l.Info("skipping init package component since external registry information was provided", "component", component.Name)
		return nil, nil
	}

	if isRegistry {
		// If we are deploying the registry then mark the HPA as "modified" to set it to Min later
		p.hpaModified = true
	}

	// Before deploying the seed registry, start the injector
	if isSeedRegistry {
		err := p.cluster.StartInjection(ctx, p.layout.Base, p.layout.Images.Base, component.Images)
		if err != nil {
			return nil, err
		}
	}

	// Skip image checksum if component is agent.
	// Skip image push if component is seed registry.
	charts, err := p.deployComponent(ctx, component, isAgent, isSeedRegistry)
	if err != nil {
		return nil, err
	}

	// Do cleanup for when we inject the seed registry during initialization
	if isSeedRegistry {
		if err := p.cluster.StopInjection(ctx); err != nil {
			return nil, fmt.Errorf("failed to delete injector resources: %w", err)
		}
	}

	return charts, nil
}

func deployComponent(ctx context.Context, component v1alpha1.ZarfComponent, noImgChecksum bool, noImgPush bool) ([]types.InstalledChart, error) {
	l := logger.From(ctx)
	start := time.Now()
	// Toggles for general deploy operations
	componentPath := p.layout.Components.Dirs[component.Name]

	message.HeaderInfof("📦 %s COMPONENT", strings.ToUpper(component.Name))
	l.Info("deploying component", "name", component.Name)

	hasImages := len(component.Images) > 0 && !noImgPush
	hasCharts := len(component.Charts) > 0
	hasManifests := len(component.Manifests) > 0
	hasRepos := len(component.Repos) > 0
	hasFiles := len(component.Files) > 0

	onDeploy := component.Actions.OnDeploy

	if component.RequiresCluster() {
		// Setup the state in the config
		if p.state == nil {
			err := p.setupState(ctx)
			if err != nil {
				return nil, err
			}
		}

		// Disable the registry HPA scale down if we are deploying images and it is not already disabled
		if hasImages && !p.hpaModified && p.state.RegistryInfo.IsInternal() {
			if err := p.cluster.DisableRegHPAScaleDown(ctx); err != nil {
				message.Debugf("unable to disable the registry HPA scale down: %s", err.Error())
				l.Debug("unable to disable the registry HPA scale down", "error", err.Error())
			} else {
				p.hpaModified = true
			}
		}
	}

	err := p.populateComponentAndStateTemplates(ctx, component.Name)
	if err != nil {
		return nil, err
	}

	if err = actions.Run(ctx, onDeploy.Defaults, onDeploy.Before, p.variableConfig); err != nil {
		return nil, fmt.Errorf("unable to run component before action: %w", err)
	}

	if hasFiles {
		if err := p.processComponentFiles(ctx, component, componentPath.Files); err != nil {
			return nil, fmt.Errorf("unable to process the component files: %w", err)
		}
	}

	if hasImages {
		if err := p.pushImagesToRegistry(ctx, component.Images, noImgChecksum); err != nil {
			return nil, fmt.Errorf("unable to push images to the registry: %w", err)
		}
	}

	if hasRepos {
		if err = p.pushReposToRepository(ctx, componentPath.Repos, component.Repos); err != nil {
			return nil, fmt.Errorf("unable to push the repos to the repository: %w", err)
		}
	}

	g, gCtx := errgroup.WithContext(ctx)
	for idx, data := range component.DataInjections {
		g.Go(func() error {
			return p.cluster.HandleDataInjection(gCtx, data, componentPath, idx)
		})
	}

	charts := []types.InstalledChart{}
	if hasCharts || hasManifests {
		charts, err = p.installChartAndManifests(ctx, componentPath, component)
		if err != nil {
			return nil, err
		}
	}

	if err = actions.Run(ctx, onDeploy.Defaults, onDeploy.After, p.variableConfig); err != nil {
		return nil, fmt.Errorf("unable to run component after action: %w", err)
	}

	if len(component.HealthChecks) > 0 {
		healthCheckContext, cancel := context.WithTimeout(ctx, p.cfg.DeployOpts.Timeout)
		defer cancel()
		spinner := message.NewProgressSpinner("Running health checks")
		l.Info("running health checks")
		defer spinner.Stop()
		if err = healthchecks.Run(healthCheckContext, p.cluster.Watcher, component.HealthChecks); err != nil {
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
