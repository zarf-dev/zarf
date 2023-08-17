// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/releaseutil"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

// Set the default helm client timeout to 15 minutes
const defaultClientTimeout = 15 * time.Minute

// Set the default number of Helm install/upgrade attempts to 3
const defaultHelmAttempts = 3

// InstallOrUpgradeChart performs a helm install or upgrade of the given chart.
func (h *HelmCfg) InstallOrUpgradeChart() (types.ConnectStrings, string, error) {
	fromMessage := h.chart.URL
	if fromMessage == "" {
		fromMessage = "Zarf-generated helm chart"
	}
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from %s",
		h.chart.Name,
		h.chart.Version,
		fromMessage)
	defer spinner.Stop()

	var output *release.Release

	h.releaseName = h.chart.ReleaseName

	// If no release name is specified, use the chart name.
	if h.releaseName == "" {
		h.releaseName = h.chart.Name
	}

	// Do not wait for the chart to be ready if data injections are present.
	if len(h.component.DataInjections) > 0 {
		spinner.Updatef("Data injections detected, not waiting for chart to be ready")
		h.chart.NoWait = true
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

	attempt := 0
	for {
		attempt++

		histClient := action.NewHistory(h.actionConfig)
		histClient.Max = 1
		releases, histErr := histClient.Run(h.releaseName)

		if attempt > 3 {
			previouslyDeployed := false

			// Check for previous releases that successfully deployed
			for _, release := range releases {
				if release.Info.Status == "deployed" {
					previouslyDeployed = true
				}
			}

			// On total failure try to rollback or uninstall.
			if previouslyDeployed {
				spinner.Updatef("Performing chart rollback")
				err = h.rollbackChart(h.releaseName)
				if err != nil {
					return nil, "", fmt.Errorf("unable to upgrade chart after %d attempts and unable to rollback: %w", defaultHelmAttempts, err)
				}

				return nil, "", fmt.Errorf("unable to upgrade chart after %d attempts", defaultHelmAttempts)
			}

			spinner.Updatef("Performing chart uninstall")
			_, err = h.uninstallChart(h.releaseName)
			if err != nil {
				return nil, "", fmt.Errorf("unable to install chart after %d attempts and unable to uninstall: %w", defaultHelmAttempts, err)
			}

			return nil, "", fmt.Errorf("unable to install chart after %d attempts", defaultHelmAttempts)
		}

		spinner.Updatef("Attempt %d of %d to install chart", attempt, defaultHelmAttempts)

		spinner.Updatef("Checking for existing helm deployment")

		if histErr == driver.ErrReleaseNotFound {
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
			return nil, "", fmt.Errorf("unable to verify the chart installation status: %w", histErr)
		}

		if err != nil {
			spinner.Errorf(err, "Unable to complete helm chart install/upgrade, waiting 10 seconds and trying again")
			// Simply wait for dust to settle and try again.
			time.Sleep(10 * time.Second)
		} else {
			message.Debug(output.Info.Description)
			spinner.Success()
			break
		}
	}

	// return any collected connect strings for zarf connect.
	return postRender.connectStrings, h.releaseName, nil
}

// TemplateChart generates a helm template from a given chart.
func (h *HelmCfg) TemplateChart() (string, chartutil.Values, error) {
	message.Debugf("helm.TemplateChart()")
	spinner := message.NewProgressSpinner("Templating helm chart %s", h.chart.Name)
	defer spinner.Stop()

	err := h.createActionConfig(h.chart.Namespace, spinner)

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
	if h.kubeVersionOverride != "" {
		parsedKubeVersion, err := chartutil.ParseKubeVersion(h.kubeVersionOverride)
		if err != nil {
			return "", nil, fmt.Errorf("invalid kube version '%s': %s", h.kubeVersionOverride, err)
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

	// Perform the loadedChart installation.
	templatedChart, err := client.Run(loadedChart, chartValues)
	if err != nil {
		return "", nil, fmt.Errorf("error generating helm chart template: %w", err)
	}

	manifest := templatedChart.Manifest

	for _, hook := range templatedChart.Hooks {
		manifest += fmt.Sprintf("\n---\n%s", hook.Manifest)
	}

	spinner.Success()

	return manifest, chartValues, nil
}

// GenerateChart generates a helm chart for a given Zarf manifest.
func (h *HelmCfg) GenerateChart(manifest types.ZarfManifest) error {
	message.Debugf("helm.GenerateChart(%#v)", manifest)
	spinner := message.NewProgressSpinner("Starting helm chart generation %s", manifest.Name)
	defer spinner.Stop()

	// Generate a new chart.
	tmpChart := new(chart.Chart)
	tmpChart.Metadata = new(chart.Metadata)

	// Generate a hashed chart name.
	rawChartName := fmt.Sprintf("raw-%s-%s-%s", h.pkgMetadata.Name, h.component.Name, manifest.Name)
	hasher := sha1.New()
	hasher.Write([]byte(rawChartName))
	tmpChart.Metadata.Name = rawChartName
	sha1ReleaseName := hex.EncodeToString(hasher.Sum(nil))

	// This is fun, increment forward in a semver-way using epoch so helm doesn't cry.
	tmpChart.Metadata.Version = fmt.Sprintf("0.1.%d", config.GetStartTime())
	tmpChart.Metadata.APIVersion = chart.APIVersionV1

	// Add the manifest files so helm does its thing.
	for _, file := range manifest.Files {
		spinner.Updatef("Processing %s", file)
		manifest := path.Join(h.componentPaths.Manifests, file)
		data, err := os.ReadFile(manifest)
		if err != nil {
			return fmt.Errorf("unable to read manifest file %s: %w", manifest, err)
		}

		// Escape all chars and then wrap in {{ }}.
		txt := strconv.Quote(string(data))
		data = []byte("{{" + txt + "}}")

		tmpChart.Templates = append(tmpChart.Templates, &chart.File{Name: manifest, Data: data})
	}

	// Generate the struct to pass to InstallOrUpgradeChart().
	h.chart = &types.ZarfChart{
		Name: tmpChart.Metadata.Name,
		// Preserve the zarf prefix for chart names to match v0.22.x and earlier behavior.
		ReleaseName: fmt.Sprintf("zarf-%s", sha1ReleaseName),
		Version:     tmpChart.Metadata.Version,
		Namespace:   manifest.Namespace,
		NoWait:      manifest.NoWait,
	}
	h.chartOverride = tmpChart

	// We don't have any values because we do not expose them in the zarf.yaml currently.
	h.valueOverride = map[string]any{}

	spinner.Success()

	return nil
}

// RemoveChart removes a chart from the cluster.
func (h *HelmCfg) RemoveChart(namespace string, name string, spinner *message.Spinner) error {
	// Establish a new actionConfig for the namespace.
	_ = h.createActionConfig(namespace, spinner)
	// Perform the uninstall.
	response, err := h.uninstallChart(name)
	message.Debug(response)
	return err
}

// UpdateReleaseValues updates values for a given chart release
// (note: this only works on single-deep charts, charts with dependencies (like loki-stack) will not work)
func (h *HelmCfg) UpdateReleaseValues(updatedValues map[string]interface{}) error {
	spinner := message.NewProgressSpinner("Updating values for helm release %s", h.releaseName)
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
	releases, histErr := histClient.Run(h.releaseName)
	if histErr == nil && len(releases) > 0 {
		lastRelease := releases[len(releases)-1]

		// Setup a new upgrade action
		client := action.NewUpgrade(h.actionConfig)

		// Let each chart run for the default timeout.
		client.Timeout = defaultClientTimeout

		client.SkipCRDs = true

		// Namespace must be specified.
		client.Namespace = h.chart.Namespace

		// Post-processing our manifests for reasons....
		client.PostRenderer = postRender

		// Set reuse values to only override the values we are explicitly given
		client.ReuseValues = true

		// Wait for the update operation to successfully complete
		client.Wait = true

		// Perform the loadedChart upgrade.
		_, err = client.Run(h.releaseName, lastRelease.Chart, updatedValues)
		if err != nil {
			return err
		}

		spinner.Success()

		return nil
	}

	return fmt.Errorf("unable to find the %s helm release", h.releaseName)
}

func (h *HelmCfg) installChart(postRender *renderer) (*release.Release, error) {
	// Bind the helm action.
	client := action.NewInstall(h.actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = defaultClientTimeout

	// Default helm behavior for Zarf is to wait for the resources to deploy, NoWait overrides that for special cases (such as data-injection).
	client.Wait = !h.chart.NoWait

	// We need to include CRDs or operator installations will fail spectacularly.
	client.SkipCRDs = false

	// Must be unique per-namespace and < 53 characters. @todo: restrict helm loadedChart name to this.
	client.ReleaseName = h.releaseName

	// Namespace must be specified.
	client.Namespace = h.chart.Namespace

	// Post-processing our manifests for reasons....
	client.PostRenderer = postRender

	loadedChart, chartValues, err := h.loadChartData()
	if err != nil {
		return nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart installation.
	return client.Run(loadedChart, chartValues)
}

func (h *HelmCfg) upgradeChart(lastRelease *release.Release, postRender *renderer) (*release.Release, error) {
	// Print the postRender object piece by piece to not print the htpasswd
	message.Debugf("helm.upgradeChart(%#v, %#v, %#v, %#v)", postRender.actionConfig, postRender.connectStrings, postRender.namespaces, postRender.options)

	// Migrate any deprecated APIs (if applicable)
	err := h.migrateDeprecatedAPIs(lastRelease)
	if err != nil {
		return nil, fmt.Errorf("unable to check for API deprecations: %w", err)
	}

	// Setup a new upgrade action
	client := action.NewUpgrade(h.actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = defaultClientTimeout

	// Default helm behavior for Zarf is to wait for the resources to deploy, NoWait overrides that for special cases (such as data-injection).
	client.Wait = !h.chart.NoWait

	client.SkipCRDs = true

	// Namespace must be specified.
	client.Namespace = h.chart.Namespace

	// Post-processing our manifests for reasons....
	client.PostRenderer = postRender

	loadedChart, chartValues, err := h.loadChartData()
	if err != nil {
		return nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart upgrade.
	return client.Run(h.releaseName, loadedChart, chartValues)
}

func (h *HelmCfg) rollbackChart(name string) error {
	message.Debugf("helm.rollbackChart(%s)", name)
	client := action.NewRollback(h.actionConfig)
	client.CleanupOnFail = true
	client.Force = true
	client.Wait = true
	client.Timeout = defaultClientTimeout
	return client.Run(name)
}

func (h *HelmCfg) uninstallChart(name string) (*release.UninstallReleaseResponse, error) {
	message.Debugf("helm.uninstallChart(%s)", name)
	client := action.NewUninstall(h.actionConfig)
	client.KeepHistory = false
	client.Wait = true
	client.Timeout = defaultClientTimeout
	return client.Run(name)
}

func (h *HelmCfg) loadChartData() (*chart.Chart, chartutil.Values, error) {
	message.Debugf("helm.loadChartData()")
	var (
		loadedChart *chart.Chart
		chartValues chartutil.Values
		err         error
	)

	if h.chartOverride == nil || h.valueOverride == nil {
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
		chartValues = h.valueOverride
	}

	return loadedChart, chartValues, nil
}

func (h *HelmCfg) migrateDeprecatedAPIs(latestRelease *release.Release) error {
	// Get the Kubernetes version from the current cluster
	kubeVersion, err := h.cluster.GetServerVersion()
	if err != nil {
		return err
	}

	kubeGitVersion, err := semver.NewVersion(kubeVersion)
	if err != nil {
		return err
	}

	// Use helm to re-split the manifest byte (same call used by helm to pass this data to postRender)
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

		rawData, manifestModified, _ := h.cluster.HandleDeprecations(rawData, *kubeGitVersion)
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
