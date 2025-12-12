// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/variables"

	"github.com/Masterminds/semver/v3"
	plutoversionsfile "github.com/fairwindsops/pluto/v5"
	plutoapi "github.com/fairwindsops/pluto/v5/pkg/api"
	goyaml "github.com/goccy/go-yaml"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
	"helm.sh/helm/v4/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
)

// Use same default as Helm CLI does.
const maxHelmHistory = 10

// InstallUpgradeOptions provide options for the Helm install/upgrade operation
type InstallUpgradeOptions struct {
	// AdoptExistingResources is true if the chart should adopt existing namespaces
	AdoptExistingResources bool
	// VariableConfig is used to template the variables in the chart
	VariableConfig *variables.VariableConfig
	// State is used to update the registry / git server secrets
	State   *state.State
	Cluster *cluster.Cluster
	// AirgapMode is true if the package being installed is not a YOLO package and it helps determine if Zarf state secrets should be updated
	AirgapMode bool
	// Timeout for the helm install/upgrade
	Timeout time.Duration
	// PkgName is the name of the zarf package being installed
	PkgName string
	// NamespaceOverride is the namespace override to use for the chart
	NamespaceOverride string
	// IsInteractive decides if Zarf can interactively prompt users through the CLI
	IsInteractive bool
}

// InstallOrUpgradeChart performs a helm install of the given chart.
func InstallOrUpgradeChart(ctx context.Context, zarfChart v1alpha1.ZarfChart, chart *chartv2.Chart, values common.Values, opts InstallUpgradeOptions) (state.ConnectStrings, string, error) {
	l := logger.From(ctx)
	start := time.Now()
	source := zarfChart.URL
	if source == "" {
		source = "Zarf-generated"
	}
	l.Info("processing Helm chart", "name", zarfChart.Name, "version", zarfChart.Version, "source", source)

	// If no release name is specified, use the chart name.
	if zarfChart.ReleaseName == "" {
		zarfChart.ReleaseName = zarfChart.Name
	}
	if opts.VariableConfig == nil {
		opts.VariableConfig = template.GetZarfVariableConfig(ctx, opts.IsInteractive)
	}

	// Setup K8s connection.
	actionConfig, err := createActionConfig(ctx, zarfChart.Namespace)
	if err != nil {
		return nil, zarfChart.ReleaseName, fmt.Errorf("unable to initialize the K8s client: %w", err)
	}

	postRender, err := newRenderer(ctx, zarfChart, opts.AdoptExistingResources, opts.Cluster, opts.AirgapMode, opts.State, actionConfig, opts.VariableConfig, opts.PkgName, opts.NamespaceOverride)
	if err != nil {
		return nil, zarfChart.ReleaseName, fmt.Errorf("unable to create helm renderer: %w", err)
	}

	histClient := action.NewHistory(actionConfig)
	var release *releasev1.Release

	helmCtx, helmCtxCancel := context.WithTimeout(ctx, opts.Timeout)
	defer helmCtxCancel()

	releases, histErr := histClient.Run(zarfChart.ReleaseName)

	l.Debug("checking for existing helm deployment")

	if errors.Is(histErr, driver.ErrReleaseNotFound) {
		// No prior release, try to install it.
		l.Info("performing Helm install", "chart", zarfChart.Name)

		release, err = installChart(helmCtx, zarfChart, chart, values, opts.Timeout, actionConfig, postRender, opts.AdoptExistingResources)
	} else if histErr == nil && len(releases) > 0 {
		// Otherwise, there is a prior release so upgrade it.
		l.Info("performing Helm upgrade", "chart", zarfChart.Name)

		lastReleaser := releases[len(releases)-1]
		// Type assert to concrete Release type
		lastRelease, ok := lastReleaser.(*releasev1.Release)
		if !ok {
			return nil, zarfChart.ReleaseName, fmt.Errorf("unable to cast release to v1.Release type")
		}

		release, err = upgradeChart(helmCtx, zarfChart, chart, values, opts.Timeout, actionConfig, postRender, opts.Cluster, lastRelease, opts.AdoptExistingResources)
	} else {
		return nil, zarfChart.ReleaseName, fmt.Errorf("unable to verify the chart installation status: %w", histErr)
	}
	if err != nil {
		removeMsg := "if you need to remove the failed chart, use `zarf package remove`"
		installErr := fmt.Errorf("unable to install chart %w: %s", err, removeMsg)

		releases, err := histClient.Run(zarfChart.ReleaseName)
		if err != nil {
			return nil, zarfChart.ReleaseName, errors.Join(err, installErr)
		}
		previouslyDeployedVersion := 0

		// Check for previous releases that successfully deployed
		for _, releaser := range releases {
			// Type assert to concrete Release type
			rel, ok := releaser.(*releasev1.Release)
			if !ok {
				continue
			}
			if rel.Info.Status == releasecommon.StatusDeployed {
				previouslyDeployedVersion = rel.Version
			}
		}

		// No prior releases means this was an initial install.
		if previouslyDeployedVersion == 0 {
			return nil, zarfChart.ReleaseName, installErr
		}

		// Attempt to rollback on a failed upgrade.
		l.Info("performing Helm rollback", "chart", zarfChart.Name)
		err = rollbackChart(zarfChart.ReleaseName, previouslyDeployedVersion, actionConfig, opts.Timeout)
		if err != nil {
			return nil, zarfChart.ReleaseName, fmt.Errorf("%w: unable to rollback: %w", installErr, err)
		}
		return nil, zarfChart.ReleaseName, installErr
	}

	resourceList, err := actionConfig.KubeClient.Build(bytes.NewBufferString(release.Manifest), true)
	if err != nil {
		return nil, zarfChart.ReleaseName, fmt.Errorf("unable to build the resource list: %w", err)
	}

	runtimeObjs := []runtime.Object{}
	for _, resource := range resourceList {
		runtimeObjs = append(runtimeObjs, resource.Object)
	}
	if !zarfChart.NoWait {
		// Ensure we don't go past the timeout by using a context initialized with the helm timeout
		l.Info("running health checks", "chart", zarfChart.Name)
		if err := healthchecks.WaitForReadyRuntime(helmCtx, opts.Cluster.Watcher, runtimeObjs); err != nil {
			return nil, zarfChart.ReleaseName, err
		}
	}
	l.Debug("done processing Helm chart", "name", zarfChart.Name, "duration", time.Since(start))

	// return any collected connect strings for zarf connect.
	return postRender.connectStrings, zarfChart.ReleaseName, nil
}

// RemoveChart removes a chart from the cluster.
func RemoveChart(ctx context.Context, namespace string, name string, timeout time.Duration) error {
	// Establish a new actionConfig for the namespace.
	actionConfig, err := createActionConfig(ctx, namespace)
	if err != nil {
		return fmt.Errorf("unable to initialize the K8s client: %w", err)
	}
	// Perform the uninstall.
	response, err := uninstallChart(name, actionConfig, timeout)
	logger.From(ctx).Debug("chart uninstalled", "response", response)
	return err
}

// UpdateReleaseValues updates values for a given chart release
// (note: this only works on single-deep charts, charts with dependencies (like loki-stack) will not work)
func UpdateReleaseValues(ctx context.Context, chart v1alpha1.ZarfChart, updatedValues map[string]interface{}, opts InstallUpgradeOptions) error {
	l := logger.From(ctx)
	l.Debug("updating values for helm release", "name", chart.ReleaseName)

	actionConfig, err := createActionConfig(ctx, chart.Namespace)
	if err != nil {
		return fmt.Errorf("unable to initialize the K8s client: %w", err)
	}
	if opts.VariableConfig == nil {
		opts.VariableConfig = template.GetZarfVariableConfig(ctx, opts.IsInteractive)
	}

	postRender, err := newRenderer(ctx, chart, opts.AdoptExistingResources, opts.Cluster, opts.AirgapMode, opts.State, actionConfig, opts.VariableConfig, opts.PkgName, opts.NamespaceOverride)
	if err != nil {
		return fmt.Errorf("unable to create helm renderer: %w", err)
	}

	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	releases, histErr := histClient.Run(chart.ReleaseName)
	if histErr == nil && len(releases) > 0 {
		lastReleaser := releases[len(releases)-1]
		// Type assert to concrete Release type
		lastRelease, ok := lastReleaser.(*releasev1.Release)
		if !ok {
			return fmt.Errorf("unable to cast release to v1.Release type")
		}

		// Setup a new upgrade action
		client := action.NewUpgrade(actionConfig)

		// FIXME: This is needed, I'm not sure why
		if lastRelease.ApplyMethod == "ssa" {
			client.ForceConflicts = true
		}

		// Let each chart run for the default timeout.
		client.Timeout = opts.Timeout

		client.SkipCRDs = true

		// Namespace must be specified.
		client.Namespace = chart.Namespace

		// Post-processing our manifests to apply vars and run zarf helm logic in cluster
		client.PostRenderer = postRender

		// Set reuse values to only override the values we are explicitly given
		client.ReuseValues = true

		// Wait for the update operation to successfully complete
		client.WaitStrategy = kube.StatusWatcherStrategy

		// Perform the loadedChart upgrade.
		_, err := client.RunWithContext(ctx, chart.ReleaseName, lastRelease.Chart, updatedValues)
		if err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("unable to find the %s helm release", chart.ReleaseName)
}

func installChart(ctx context.Context, zarfChart v1alpha1.ZarfChart, chart *chartv2.Chart, chartValues common.Values,
	timeout time.Duration, actionConfig *action.Configuration, postRender *renderer, adoptExistingResources bool) (*releasev1.Release, error) {
	// Bind the helm action.
	client := action.NewInstall(actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = timeout

	// Default helm behavior for Zarf is to wait for the resources to deploy, NoWait overrides that for special cases (such as data-injection).
	if zarfChart.NoWait {
		client.WaitStrategy = kube.HookOnlyStrategy
	} else {
		client.WaitStrategy = kube.StatusWatcherStrategy
	}

	// Force conflicts to handle Helm 3 -> Helm 4 migration (server-side apply field ownership)
	// This can only be enabled when ssa is enabled
	client.ForceConflicts = adoptExistingResources

	// We need to include CRDs or operator installations will fail spectacularly.
	client.SkipCRDs = false

	// Must be unique per-namespace and < 53 characters. @todo: restrict helm loadedChart name to this.
	client.ReleaseName = zarfChart.ReleaseName

	client.SkipSchemaValidation = !zarfChart.ShouldRunSchemaValidation()

	// Namespace must be specified.
	client.Namespace = zarfChart.Namespace

	// Post-processing our manifests to apply vars and run zarf helm logic in cluster
	client.PostRenderer = postRender

	// FIXME: do we want to expose the option to automatically do server side apply. What are the implications?
	client.ServerSideApply = true

	// Perform the loadedChart installation.
	releaser, err := client.RunWithContext(ctx, chart, chartValues)
	if err != nil {
		return nil, err
	}

	// Type assert to concrete Release type
	release, ok := releaser.(*releasev1.Release)
	if !ok {
		return nil, fmt.Errorf("unable to cast release to v1.Release type")
	}
	return release, nil
}

func upgradeChart(ctx context.Context, zarfChart v1alpha1.ZarfChart, chart *chartv2.Chart, chartValues common.Values,
	timeout time.Duration, actionConfig *action.Configuration, postRender *renderer, c *cluster.Cluster, lastRelease *releasev1.Release, adoptExistingResources bool) (*releasev1.Release, error) {
	// Migrate any deprecated APIs (if applicable)
	err := migrateDeprecatedAPIs(ctx, c, actionConfig, lastRelease)
	if err != nil {
		return nil, fmt.Errorf("unable to check for API deprecations: %w", err)
	}

	// Setup a new upgrade action
	client := action.NewUpgrade(actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = timeout

	// Default helm behavior for Zarf is to wait for the resources to deploy, NoWait overrides that for special cases (such as data-injection).
	if zarfChart.NoWait {
		client.WaitStrategy = kube.HookOnlyStrategy
	} else {
		client.WaitStrategy = kube.StatusWatcherStrategy
	}

	// FIXME: Server-side apply is causing "metadata.managedFields must be nil" errors in Helm 4
	// Temporarily disabling until we can root cause the issue
	client.ServerSideApply = "false"

	// FIXME: Need to decide if we'll keep this, most likely we will
	// Another option is to set this to adoptExistingResources
	// Not sure why this is failing. For instance during `zarf tools update-creds`
	// This can only be enabled when ssa is enabled
	if lastRelease.ApplyMethod == "ssa" {
		client.ForceConflicts = adoptExistingResources
	}

	client.SkipCRDs = true

	client.SkipSchemaValidation = !zarfChart.ShouldRunSchemaValidation()

	// Namespace must be specified.
	client.Namespace = zarfChart.Namespace

	// Post-processing our manifests to apply vars and run zarf helm logic in cluster
	client.PostRenderer = postRender

	client.MaxHistory = maxHelmHistory

	// Enable TakeOwnership when adopting existing resources
	// This tells Helm to adopt resources without Helm annotations (i.e., resources created by kubectl)
	if adoptExistingResources {
		client.TakeOwnership = true
	}

	// Perform the loadedChart upgrade.
	releaser, err := client.RunWithContext(ctx, zarfChart.ReleaseName, chart, chartValues)
	if err != nil {
		return nil, err
	}

	// Type assert to concrete Release type
	release, ok := releaser.(*releasev1.Release)
	if !ok {
		return nil, fmt.Errorf("unable to cast release to v1.Release type")
	}
	return release, nil
}

func rollbackChart(name string, version int, actionConfig *action.Configuration, timeout time.Duration) error {
	client := action.NewRollback(actionConfig)
	client.CleanupOnFail = true
	// client.ForceReplace = true
	client.ServerSideApply = "auto"
	client.WaitStrategy = kube.StatusWatcherStrategy
	client.Timeout = timeout
	client.Version = version
	client.MaxHistory = maxHelmHistory
	return client.Run(name)
}

func uninstallChart(name string, actionConfig *action.Configuration, timeout time.Duration) (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(actionConfig)
	client.KeepHistory = false
	client.WaitStrategy = kube.StatusWatcherStrategy
	client.Timeout = timeout
	return client.Run(name)
}

// LoadChartData loads a chart from a tarball and returns the Helm SDK representation of the chart and it's values
func LoadChartData(zarfChart v1alpha1.ZarfChart, chartPath string, valuesPath string, valuesOverrides map[string]any) (*chartv2.Chart, common.Values, error) {
	loadedChart, err := loadChartFromTarball(zarfChart, chartPath)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load chart tarball: %w", err)
	}

	chartValues, err := parseChartValues(zarfChart, valuesPath, valuesOverrides)
	if err != nil {
		return loadedChart, nil, fmt.Errorf("unable to parse chart values: %w", err)
	}
	return loadedChart, chartValues, nil
}

// migrateDeprecatedAPIs searches through all the objects from the latest release and migrates any deprecated APIs to the latest version.
// If any deprecated fields are found, the release will be updated and saved back to the cluster.
func migrateDeprecatedAPIs(ctx context.Context, c *cluster.Cluster, actionConfig *action.Configuration, latestRelease *releasev1.Release) error {
	// Get the Kubernetes version from the current cluster
	kubeVersion, err := c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return err
	}

	kubeGitVersion, err := semver.NewVersion(kubeVersion.String())
	if err != nil {
		return err
	}

	// Use helm to re-split the manifest bytes (same call used by helm to pass this data to postRender)
	_, resources, err := releaseutil.SortManifests(map[string]string{"manifest": latestRelease.Manifest}, nil, releaseutil.InstallOrder)

	if err != nil {
		return fmt.Errorf("error re-rendering helm output: %w", err)
	}

	modifiedManifest := ""
	modified := false

	// Loop over the resources from the lastRelease manifest to check for deprecations
	for _, resource := range resources {
		// parse to unstructured to have access to more data than just the name
		rawData := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(resource.Content), rawData); err != nil {
			return fmt.Errorf("failed to unmarshal manifest: %w", err)
		}

		rawData, manifestModified, err := handleDeprecations(rawData, *kubeGitVersion)
		if err != nil {
			// avoid returning the err here in case pluto uses an invalid semver
			logger.From(ctx).Error("unable to update deprecated resource", "name", resource.Name, "err", err.Error())
		}
		manifestContent, err := yaml.Marshal(rawData)
		if err != nil {
			return fmt.Errorf("failed to marshal raw manifest after deprecation check: %w", err)
		}

		// If this is not a bad object, place it back into the manifest
		modifiedManifest += fmt.Sprintf("---\n# Source: %s\n%s\n", resource.Name, manifestContent)

		if manifestModified {
			modified = true
		}
	}

	// If the release was modified in the above loop, save it back to the cluster
	if modified {
		logger.From(ctx).Warn("detected deprecated APIs for the helm release", "name", latestRelease.Name)

		// Update current release version to be superseded (same as the helm mapkubeapis plugin)
		latestRelease.Info.Status = releasecommon.StatusSuperseded
		if err := actionConfig.Releases.Update(latestRelease); err != nil {
			return err
		}

		// Use a shallow copy of current release version to update the object with the modification
		// and then store this new version (same as the helm mapkubeapis plugin)
		var newRelease = latestRelease
		newRelease.Manifest = modifiedManifest
		newRelease.Info.Description = "Kubernetes deprecated API upgrade - DO NOT rollback from this version"
		newRelease.Info.LastDeployed = actionConfig.Now()
		newRelease.Version = latestRelease.Version + 1
		newRelease.Info.Status = releasecommon.StatusDeployed
		if err := actionConfig.Releases.Create(newRelease); err != nil {
			return err
		}
	}

	return nil
}

// handleDeprecations takes in an unstructured object and the k8s version and returns the latest version of the object and if it was modified.
func handleDeprecations(rawData *unstructured.Unstructured, kubernetesVersion semver.Version) (*unstructured.Unstructured, bool, error) {
	deprecatedVersionContent := &plutoapi.VersionFile{}
	err := goyaml.Unmarshal(plutoversionsfile.Content(), deprecatedVersionContent)
	if err != nil {
		return rawData, false, err
	}
	for _, deprecation := range deprecatedVersionContent.DeprecatedVersions {
		if deprecation.Component == "k8s" && deprecation.Kind == rawData.GetKind() && deprecation.Name == rawData.GetAPIVersion() {
			removedVersion, err := semver.NewVersion(deprecation.RemovedIn)
			if err != nil {
				return rawData, false, err
			}

			if removedVersion.LessThan(&kubernetesVersion) {
				if deprecation.ReplacementAPI != "" {
					rawData.SetAPIVersion(deprecation.ReplacementAPI)
					return rawData, true, nil
				}

				return nil, true, nil
			}
		}
	}
	return rawData, false, nil
}
