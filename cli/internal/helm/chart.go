package helm

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type ChartOptions struct {
	BasePath string
	Chart    config.ZarfChart
	Images   []string
}

type renderer struct {
	images []string
}

// InstallOrUpgradeChart performs a helm install of the given chart
func InstallOrUpgradeChart(options ChartOptions) {
	spinner := message.NewProgresSpinner("Processing helm chart %s:%s from %s",
		options.Chart.Name,
		options.Chart.Version,
		options.Chart.Url)
	defer spinner.Stop()

	k8s.ReplaceRegistrySecret(options.Chart.Namespace)

	var output *release.Release

	actionConfig, err := createActionConfig(options.Chart.Namespace)

	// Setup K8s connection
	if err != nil {
		spinner.Fatalf(err, "Unable to initialize the K8s client")
	}

	attempt := 0
	for {
		attempt++

		spinner.Updatef("Attempt %d of 3 to install chart", attempt)
		histClient := action.NewHistory(actionConfig)
		histClient.Max = 1

		if attempt > 2 {
			// On total failure try to rollback or uninstall
			if histClient.Version > 1 {
				spinner.Updatef("Performing chart rollback")
				_ = rollbackChart(actionConfig, options.Chart.Name)
			} else {
				spinner.Updatef("Performing chart uninstall")
				_, _ = uninstallChart(actionConfig, options.Chart.Name)
			}
			spinner.Errorf(nil, "Unable to complete helm chart install/upgrade")
			break
		}

		spinner.Updatef("Checking for existing helm deployment")
		if _, histErr := histClient.Run(options.Chart.Name); histErr == driver.ErrReleaseNotFound {
			// No prior release, try to install it
			spinner.Updatef("Attempting chart installation")
			output, err = installChart(actionConfig, options)
		} else if err != nil {
			// Something broke
			spinner.Fatalf(err, "Unable to verify the chart installation status")
		} else {
			// Otherwise, there is a prior release so upgrade it
			spinner.Updatef("Attempting chart upgrade")
			output, err = upgradeChart(actionConfig, options)
		}

		if err != nil {
			spinner.Debugf(err.Error())
			// Simply wait for dust to settle and try again
			time.Sleep(10 * time.Second)
		} else {
			spinner.Debugf(output.Info.Description)
			spinner.Success()
			break
		}

	}
}

// TemplateChart generates a helm template from a given chart
func TemplateChart(options ChartOptions) string {
	spinner := message.NewProgresSpinner("Processing helm template %s:%s from %s",
		options.Chart.Name,
		options.Chart.Version,
		options.Chart.Url)
	defer spinner.Stop()

	k8s.ReplaceRegistrySecret(options.Chart.Namespace)

	actionConfig, err := createActionConfig(options.Chart.Namespace)

	// Setup K8s connection
	if err != nil {
		spinner.Fatalf(err, "Unable to initialize the K8s client")
	}

	// Bind the helm action
	client := action.NewInstall(actionConfig)

	client.DryRun = false
	client.Replace = true // Skip the name check
	client.ClientOnly = true
	client.IncludeCRDs = true

	// Must be unique per-namespace and < 53 characters. @todo: restrict helm loadedChart name to this
	client.ReleaseName = options.Chart.Name

	// Namespace must be specified
	client.Namespace = options.Chart.Namespace

	loadedChart, chartValues, err := loadChartData(options)
	if err != nil {
		spinner.Fatalf(err, "unable to load chart data")
	}

	// Perform the loadedChart installation
	templatedChart, err := client.Run(loadedChart, chartValues)

	if err != nil {
		spinner.Fatalf(err, "Unable to install the helm loadedChart")
	} else {
		spinner.Debugf(templatedChart.Manifest)
	}

	spinner.Success()

	return templatedChart.Manifest
}

func installChart(actionConfig *action.Configuration, options ChartOptions) (*release.Release, error) {
	// Bind the helm action
	client := action.NewInstall(actionConfig)

	// Let each chart run for 5 minutes
	client.Timeout = 15 * time.Minute

	client.Wait = true

	// We need to include CRDs or operator installations will fail spectacularly
	client.SkipCRDs = false

	// Must be unique per-namespace and < 53 characters. @todo: restrict helm loadedChart name to this
	client.ReleaseName = options.Chart.Name

	// Namespace must be specified
	client.Namespace = options.Chart.Namespace

	// Post-processing our manifests for reasons....
	client.PostRenderer = NewRenderer(options.Images)

	loadedChart, chartValues, err := loadChartData(options)
	if err != nil {
		return nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart installation
	return client.Run(loadedChart, chartValues)
}

func upgradeChart(actionConfig *action.Configuration, options ChartOptions) (*release.Release, error) {
	client := action.NewUpgrade(actionConfig)

	// Let each chart run for 5 minutes
	client.Timeout = 10 * time.Minute

	client.Wait = true

	client.SkipCRDs = true

	// Namespace must be specified
	client.Namespace = options.Chart.Namespace

	// Post-processing our manifests for reasons....
	client.PostRenderer = NewRenderer(options.Images)

	loadedChart, chartValues, err := loadChartData(options)
	if err != nil {
		return nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart upgrade
	return client.Run(options.Chart.Name, loadedChart, chartValues)
}

func rollbackChart(actionConfig *action.Configuration, name string) error {
	client := action.NewRollback(actionConfig)
	client.CleanupOnFail = true
	client.Force = true
	client.Wait = true
	client.Timeout = 1 * time.Minute
	return client.Run(name)
}
func uninstallChart(actionConfig *action.Configuration, name string) (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(actionConfig)
	client.KeepHistory = false
	client.Timeout = 3 * time.Minute
	client.Wait = true
	return client.Run(name)
}

func loadChartData(options ChartOptions) (*chart.Chart, map[string]interface{}, error) {
	loadedChart, err := loadChartFromTarball(options)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load chart tarball: %w", err)
	}

	chartValues, err := parseChartValues(options)
	if err != nil {
		return loadedChart, nil, fmt.Errorf("unable to parse chart values: %w", err)
	}
	message.Debug(chartValues)

	return loadedChart, chartValues, nil
}

func NewRenderer(images []string) *renderer {
	return &renderer{
		images: images,
	}
}

func (r *renderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	message.Debug("Post-rendering helm chart")
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	tempDir, _ := utils.MakeTempDir()
	path := tempDir + "/chart.yaml"

	utils.WriteFile(path, renderedManifests.Bytes())

	// Run the template engine against the chart output
	k8s.ProcessYamlFilesInPath(tempDir, r.images)

	// Read back the final file contents
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading temporary post-rendered helm chart: %w", err)
	}

	message.Debug(string(buff))

	// Cleanup the temp file
	_ = os.RemoveAll(tempDir)

	// Send the bytes back to helm
	return bytes.NewBuffer(buff), nil
}
