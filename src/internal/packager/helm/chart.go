// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	plutoversionsfile "github.com/fairwindsops/pluto/v5"
	plutoapi "github.com/fairwindsops/pluto/v5/pkg/api"
	goyaml "github.com/goccy/go-yaml"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/defenseunicorns/pkg/helpers/v2"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/types"
)

// InstallOrUpgradeChart performs a helm install of the given chart.
func (h *Helm) InstallOrUpgradeChart() (types.ConnectStrings, string, error) {
	fromMessage := h.chart.URL
	if fromMessage == "" {
		fromMessage = "Zarf-generated helm chart"
	}
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from %s",
		h.chart.Name,
		h.chart.Version,
		fromMessage)
	defer spinner.Stop()

	// If no release name is specified, use the chart name.
	if h.chart.ReleaseName == "" {
		h.chart.ReleaseName = h.chart.Name
	}

	// Setup K8s connection.
	err := h.createActionConfig(h.chart.Namespace, spinner)
	if err != nil {
		return nil, "", fmt.Errorf("unable to initialize the K8s client: %w", err)
	}

	postRender, err := h.newRenderer()
	if err != nil {
		return nil, "", fmt.Errorf("unable to create helm renderer: %w", err)
	}

	histClient := action.NewHistory(h.actionConfig)
	tryHelm := func() error {
		var err error
		var output *release.Release

		releases, histErr := histClient.Run(h.chart.ReleaseName)

		spinner.Updatef("Checking for existing helm deployment")

		if errors.Is(histErr, driver.ErrReleaseNotFound) {
			// No prior release, try to install it.
			spinner.Updatef("Attempting chart installation")

			output, err = h.installChart(postRender)
		} else if histErr == nil && len(releases) > 0 {
			// Otherwise, there is a prior release so upgrade it.
			spinner.Updatef("Attempting chart upgrade")

			lastRelease := releases[len(releases)-1]

			output, err = h.upgradeChart(lastRelease, postRender)
		} else {
			// 😭 things aren't working
			return fmt.Errorf("unable to verify the chart installation status: %w", histErr)
		}

		if err != nil {
			return err
		}

		message.Debug(output.Info.Description)
		spinner.Success()
		return nil
	}

	err = helpers.Retry(tryHelm, h.retries, 5*time.Second, message.Warnf)
	if err != nil {
		releases, _ := histClient.Run(h.chart.ReleaseName)
		previouslyDeployedVersion := 0

		// Check for previous releases that successfully deployed
		for _, release := range releases {
			if release.Info.Status == "deployed" {
				previouslyDeployedVersion = release.Version
			}
		}

		removeMsg := "if you need to remove the failed chart, use `zarf package remove`"

		// No prior releases means this was an initial install.
		if previouslyDeployedVersion == 0 {
			return nil, "", fmt.Errorf("unable to install chart after %d attempts: %s", h.retries, removeMsg)
		}

		// Attempt to rollback on a failed upgrade.
		spinner.Updatef("Performing chart rollback")
		err = h.rollbackChart(h.chart.ReleaseName, previouslyDeployedVersion)
		if err != nil {
			return nil, "", fmt.Errorf("unable to upgrade chart after %d attempts and unable to rollback: %s", h.retries, removeMsg)
		}

		return nil, "", fmt.Errorf("unable to upgrade chart after %d attempts: %s", h.retries, removeMsg)
	}

	// return any collected connect strings for zarf connect.
	return postRender.connectStrings, h.chart.ReleaseName, nil
}

// TemplateChart generates a helm template from a given chart.
func (h *Helm) TemplateChart() (manifest string, chartValues chartutil.Values, err error) {
	message.Debugf("helm.TemplateChart()")
	spinner := message.NewProgressSpinner("Templating helm chart %s", h.chart.Name)
	defer spinner.Stop()

	err = h.createActionConfig(h.chart.Namespace, spinner)

	// Setup K8s connection.
	if err != nil {
		return "", nil, fmt.Errorf("unable to initialize the K8s client: %w", err)
	}

	// Bind the helm action.
	client := action.NewInstall(h.actionConfig)

	client.DryRun = true
	client.Replace = true // Skip the name check.
	client.ClientOnly = true
	client.IncludeCRDs = true
	// TODO: Further research this with regular/OCI charts
	client.Verify = false
	client.InsecureSkipTLSverify = config.CommonOptions.Insecure
	if h.kubeVersion != "" {
		parsedKubeVersion, err := chartutil.ParseKubeVersion(h.kubeVersion)
		if err != nil {
			return "", nil, fmt.Errorf("invalid kube version '%s': %s", h.kubeVersion, err)
		}
		client.KubeVersion = parsedKubeVersion
	}
	client.ReleaseName = h.chart.ReleaseName

	// If no release name is specified, use the chart name.
	if client.ReleaseName == "" {
		client.ReleaseName = h.chart.Name
	}

	// Namespace must be specified.
	client.Namespace = h.chart.Namespace

	loadedChart, chartValues, err := h.loadChartData()
	if err != nil {
		return "", nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	client.PostRenderer, err = h.newRenderer()
	if err != nil {
		return "", nil, fmt.Errorf("unable to create helm renderer: %w", err)
	}

	// Perform the loadedChart installation.
	templatedChart, err := client.Run(loadedChart, chartValues)
	if err != nil {
		return "", nil, fmt.Errorf("error generating helm chart template: %w", err)
	}

	manifest = templatedChart.Manifest

	for _, hook := range templatedChart.Hooks {
		manifest += fmt.Sprintf("\n---\n%s", hook.Manifest)
	}

	spinner.Success()

	return manifest, chartValues, nil
}

// RemoveChart removes a chart from the cluster.
func (h *Helm) RemoveChart(namespace string, name string, spinner *message.Spinner) error {
	// Establish a new actionConfig for the namespace.
	_ = h.createActionConfig(namespace, spinner)
	// Perform the uninstall.
	response, err := h.uninstallChart(name)
	message.Debug(response)
	return err
}

// UpdateReleaseValues updates values for a given chart release
// (note: this only works on single-deep charts, charts with dependencies (like loki-stack) will not work)
func (h *Helm) UpdateReleaseValues(updatedValues map[string]interface{}) error {
	spinner := message.NewProgressSpinner("Updating values for helm release %s", h.chart.ReleaseName)
	defer spinner.Stop()

	err := h.createActionConfig(h.chart.Namespace, spinner)
	if err != nil {
		return fmt.Errorf("unable to initialize the K8s client: %w", err)
	}

	postRender, err := h.newRenderer()
	if err != nil {
		return fmt.Errorf("unable to create helm renderer: %w", err)
	}

	histClient := action.NewHistory(h.actionConfig)
	histClient.Max = 1
	releases, histErr := histClient.Run(h.chart.ReleaseName)
	if histErr == nil && len(releases) > 0 {
		lastRelease := releases[len(releases)-1]

		// Setup a new upgrade action
		client := action.NewUpgrade(h.actionConfig)

		// Let each chart run for the default timeout.
		client.Timeout = h.timeout

		client.SkipCRDs = true

		// Namespace must be specified.
		client.Namespace = h.chart.Namespace

		// Post-processing our manifests to apply vars and run zarf helm logic in cluster
		client.PostRenderer = postRender

		// Set reuse values to only override the values we are explicitly given
		client.ReuseValues = true

		// Wait for the update operation to successfully complete
		client.Wait = true

		// Perform the loadedChart upgrade.
		_, err = client.Run(h.chart.ReleaseName, lastRelease.Chart, updatedValues)
		if err != nil {
			return err
		}

		spinner.Success()

		return nil
	}

	return fmt.Errorf("unable to find the %s helm release", h.chart.ReleaseName)
}

func (h *Helm) installChart(postRender *renderer) (*release.Release, error) {
	// Bind the helm action.
	client := action.NewInstall(h.actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = h.timeout

	// Default helm behavior for Zarf is to wait for the resources to deploy, NoWait overrides that for special cases (such as data-injection).
	client.Wait = !h.chart.NoWait

	// We need to include CRDs or operator installations will fail spectacularly.
	client.SkipCRDs = false

	// Must be unique per-namespace and < 53 characters. @todo: restrict helm loadedChart name to this.
	client.ReleaseName = h.chart.ReleaseName

	// Namespace must be specified.
	client.Namespace = h.chart.Namespace

	// Post-processing our manifests to apply vars and run zarf helm logic in cluster
	client.PostRenderer = postRender

	loadedChart, chartValues, err := h.loadChartData()
	if err != nil {
		return nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart installation.
	return client.Run(loadedChart, chartValues)
}

func (h *Helm) upgradeChart(lastRelease *release.Release, postRender *renderer) (*release.Release, error) {
	// Migrate any deprecated APIs (if applicable)
	err := h.migrateDeprecatedAPIs(lastRelease)
	if err != nil {
		return nil, fmt.Errorf("unable to check for API deprecations: %w", err)
	}

	// Setup a new upgrade action
	client := action.NewUpgrade(h.actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = h.timeout

	// Default helm behavior for Zarf is to wait for the resources to deploy, NoWait overrides that for special cases (such as data-injection).
	client.Wait = !h.chart.NoWait

	client.SkipCRDs = true

	// Namespace must be specified.
	client.Namespace = h.chart.Namespace

	// Post-processing our manifests to apply vars and run zarf helm logic in cluster
	client.PostRenderer = postRender

	loadedChart, chartValues, err := h.loadChartData()
	if err != nil {
		return nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart upgrade.
	return client.Run(h.chart.ReleaseName, loadedChart, chartValues)
}

func (h *Helm) rollbackChart(name string, version int) error {
	message.Debugf("helm.rollbackChart(%s)", name)
	client := action.NewRollback(h.actionConfig)
	client.CleanupOnFail = true
	client.Force = true
	client.Wait = true
	client.Timeout = h.timeout
	client.Version = version
	return client.Run(name)
}

func (h *Helm) uninstallChart(name string) (*release.UninstallReleaseResponse, error) {
	message.Debugf("helm.uninstallChart(%s)", name)
	client := action.NewUninstall(h.actionConfig)
	client.KeepHistory = false
	client.Wait = true
	client.Timeout = h.timeout
	return client.Run(name)
}

func (h *Helm) loadChartData() (*chart.Chart, chartutil.Values, error) {
	message.Debugf("helm.loadChartData()")
	var (
		loadedChart *chart.Chart
		chartValues chartutil.Values
		err         error
	)

	if h.chartOverride == nil {
		// If there is no override, get the chart and values info.
		loadedChart, err = h.loadChartFromTarball()
		if err != nil {
			return nil, nil, fmt.Errorf("unable to load chart tarball: %w", err)
		}

		chartValues, err = h.parseChartValues()
		if err != nil {
			return loadedChart, nil, fmt.Errorf("unable to parse chart values: %w", err)
		}
	} else {
		// Otherwise, use the overrides instead.
		loadedChart = h.chartOverride
		chartValues = h.valuesOverrides
	}

	return loadedChart, chartValues, nil
}

func (h *Helm) migrateDeprecatedAPIs(latestRelease *release.Release) error {
	// Get the Kubernetes version from the current cluster
	kubeVersion, err := h.cluster.Clientset.Discovery().ServerVersion()
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
			return fmt.Errorf("failed to unmarshal manifest: %#v", err)
		}

		rawData, manifestModified, _ := handleDeprecations(rawData, *kubeGitVersion)
		manifestContent, err := yaml.Marshal(rawData)
		if err != nil {
			return fmt.Errorf("failed to marshal raw manifest after deprecation check: %#v", err)
		}

		// If this is not a bad object, place it back into the manifest
		modifiedManifest += fmt.Sprintf("---\n# Source: %s\n%s\n", resource.Name, manifestContent)

		if manifestModified {
			modified = true
		}
	}

	// If the release was modified in the above loop, save it back to the cluster
	if modified {
		message.Warnf("Zarf detected deprecated APIs for the '%s' helm release.  Attempting automatic upgrade.", latestRelease.Name)

		// Update current release version to be superseded (same as the helm mapkubeapis plugin)
		latestRelease.Info.Status = release.StatusSuperseded
		if err := h.actionConfig.Releases.Update(latestRelease); err != nil {
			return err
		}

		// Use a shallow copy of current release version to update the object with the modification
		// and then store this new version (same as the helm mapkubeapis plugin)
		var newRelease = latestRelease
		newRelease.Manifest = modifiedManifest
		newRelease.Info.Description = "Kubernetes deprecated API upgrade - DO NOT rollback from this version"
		newRelease.Info.LastDeployed = h.actionConfig.Now()
		newRelease.Version = latestRelease.Version + 1
		newRelease.Info.Status = release.StatusDeployed
		if err := h.actionConfig.Releases.Create(newRelease); err != nil {
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
