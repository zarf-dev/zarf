package helm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/types"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type ConnectStrings map[string]string
type ChartOptions struct {
	BasePath          string
	Chart             types.ZarfChart
	ReleaseName       string
	ChartLoadOverride string
	ChartOverride     *chart.Chart
	ValueOverride     map[string]interface{}
	Images            []string
}

type renderer struct {
	options        ChartOptions
	namespaces     []string
	connectStrings ConnectStrings
}

// InstallOrUpgradeChart performs a helm install of the given chart
func InstallOrUpgradeChart(options ChartOptions) ConnectStrings {
	spinner := message.NewProgressSpinner("Processing helm chart %s:%s from %s",
		options.Chart.Name,
		options.Chart.Version,
		options.Chart.Url)
	defer spinner.Stop()

	var output *release.Release

	options.ReleaseName = fmt.Sprintf("zarf-%s", options.Chart.Name)
	actionConfig, err := createActionConfig(options.Chart.Namespace)
	postRender := NewRenderer(options)

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
				_ = rollbackChart(actionConfig, options.ReleaseName)
			} else {
				spinner.Updatef("Performing chart uninstall")
				_, _ = uninstallChart(actionConfig, options.ReleaseName)
			}
			spinner.Errorf(nil, "Unable to complete helm chart install/upgrade")
			break
		}

		spinner.Updatef("Checking for existing helm deployment")

		_, histErr := histClient.Run(options.ReleaseName)

		switch histErr {
		case driver.ErrReleaseNotFound:
			// No prior release, try to install it
			spinner.Updatef("Attempting chart installation")
			output, err = installChart(actionConfig, options, postRender)

		case nil:
			// Otherwise, there is a prior release so upgrade it
			spinner.Updatef("Attempting chart upgrade")
			output, err = upgradeChart(actionConfig, options, postRender)

		default:
			// 😭 things aren't working
			spinner.Fatalf(err, "Unable to verify the chart installation status")
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

	// return any collected connect strings for zarf connect
	return postRender.connectStrings
}

// TemplateChart generates a helm template from a given chart
func TemplateChart(options ChartOptions) (string, error) {
	message.Debugf("helm.TemplateChart(%v)", options)

	actionConfig, err := createActionConfig(options.Chart.Namespace)

	// Setup K8s connection
	if err != nil {
		return "", fmt.Errorf("unable to initialize the K8s client: %w", err)
	}

	// Bind the helm action
	client := action.NewInstall(actionConfig)

	client.DryRun = true
	client.Replace = true // Skip the name check
	client.ClientOnly = true
	client.IncludeCRDs = true

	client.ReleaseName = fmt.Sprintf("zarf-%s", options.Chart.Name)

	// Namespace must be specified
	client.Namespace = options.Chart.Namespace

	loadedChart, chartValues, err := loadChartData(options)
	if err != nil {
		return "", fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart installation
	templatedChart, err := client.Run(loadedChart, chartValues)
	if err != nil {
		return "", fmt.Errorf("error generating helm chart template: %w", err)
	}

	return templatedChart.Manifest, nil
}

func GenerateChart(basePath string, manifest types.ZarfManifest, images []string) ConnectStrings {
	message.Debugf("helm.GenerateChart(%s, %v, %v)", basePath, manifest, images)
	spinner := message.NewProgressSpinner("Starting helm chart generation %s", manifest.Name)
	defer spinner.Stop()

	// Use timestamp to help make a valid semver
	now := time.Now()

	// Generate a new chart
	tmpChart := new(chart.Chart)
	tmpChart.Metadata = new(chart.Metadata)
	tmpChart.Metadata.Name = fmt.Sprintf("raw-%s", manifest.Name)
	// This is fun, increment forward in a semver-way using epoch so helm doesn't cry
	tmpChart.Metadata.Version = fmt.Sprintf("0.1.%d", now.Unix())
	tmpChart.Metadata.APIVersion = chart.APIVersionV1

	// Add the manifest files so helm does its thing
	for _, file := range manifest.Files {
		spinner.Updatef("Processing %s", file)
		manifest := fmt.Sprintf("%s/%s", basePath, file)
		data, err := ioutil.ReadFile(manifest)
		if err != nil {
			spinner.Fatalf(err, "Unable to read the manifest file contents")
		}
		tmpChart.Templates = append(tmpChart.Templates, &chart.File{Name: manifest, Data: data})
	}

	if manifest.DefaultNamespace == "" {
		// Helm gets sad when you don't provide a namespace even though we aren't using helm templating
		manifest.DefaultNamespace = "zarf"
	}

	// Generate the struct to pass to InstallOrUpgradeChart()
	options := ChartOptions{
		BasePath: basePath,
		Chart: types.ZarfChart{
			Name:      tmpChart.Metadata.Name,
			Version:   tmpChart.Metadata.Version,
			Namespace: manifest.DefaultNamespace,
		},
		ChartOverride: tmpChart,
		// We don't have any values because we do not expose them in the zarf.yaml currently
		ValueOverride: map[string]interface{}{},
		// Images needed for eventual post-render templating
		Images: images,
	}

	spinner.Success()

	return InstallOrUpgradeChart(options)
}

func installChart(actionConfig *action.Configuration, options ChartOptions, postRender *renderer) (*release.Release, error) {
	message.Debugf("helm.installChart(%v, %v, %v)", actionConfig, options, postRender)
	// Bind the helm action
	client := action.NewInstall(actionConfig)

	// Let each chart run for 5 minutes
	client.Timeout = 15 * time.Minute

	client.Wait = true

	// We need to include CRDs or operator installations will fail spectacularly
	client.SkipCRDs = false

	// Must be unique per-namespace and < 53 characters. @todo: restrict helm loadedChart name to this
	client.ReleaseName = options.ReleaseName

	// Namespace must be specified
	client.Namespace = options.Chart.Namespace

	// Create namespace if it does not exist
	client.CreateNamespace = true

	// Post-processing our manifests for reasons....
	client.PostRenderer = postRender

	loadedChart, chartValues, err := loadChartData(options)
	if err != nil {
		return nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart installation
	return client.Run(loadedChart, chartValues)
}

func upgradeChart(actionConfig *action.Configuration, options ChartOptions, postRender *renderer) (*release.Release, error) {
	message.Debugf("helm.upgradeChart(%v, %v, %v)", actionConfig, options, postRender)
	client := action.NewUpgrade(actionConfig)

	// Let each chart run for 5 minutes
	client.Timeout = 10 * time.Minute

	client.Wait = true

	client.SkipCRDs = true

	// Namespace must be specified
	client.Namespace = options.Chart.Namespace

	// Post-processing our manifests for reasons....
	client.PostRenderer = postRender

	loadedChart, chartValues, err := loadChartData(options)
	if err != nil {
		return nil, fmt.Errorf("unable to load chart data: %w", err)
	}

	// Perform the loadedChart upgrade
	return client.Run(options.ReleaseName, loadedChart, chartValues)
}

func rollbackChart(actionConfig *action.Configuration, name string) error {
	message.Debugf("helm.rollbackChart(%v, %s)", actionConfig, name)
	client := action.NewRollback(actionConfig)
	client.CleanupOnFail = true
	client.Force = true
	client.Wait = true
	client.Timeout = 1 * time.Minute
	return client.Run(name)
}

func uninstallChart(actionConfig *action.Configuration, name string) (*release.UninstallReleaseResponse, error) {
	message.Debugf("helm.uninstallChart(%v, %s)", actionConfig, name)
	client := action.NewUninstall(actionConfig)
	client.KeepHistory = false
	client.Timeout = 3 * time.Minute
	client.Wait = true
	return client.Run(name)
}

func loadChartData(options ChartOptions) (*chart.Chart, map[string]interface{}, error) {
	message.Debugf("helm.loadChartData(%v)", options)
	var (
		loadedChart *chart.Chart
		chartValues map[string]interface{}
		err         error
	)

	if options.ChartOverride == nil || options.ValueOverride == nil {
		// If there is no override, get the chart and values info
		loadedChart, err = loadChartFromTarball(options)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to load chart tarball: %w", err)
		}

		chartValues, err = parseChartValues(options)
		if err != nil {
			return loadedChart, nil, fmt.Errorf("unable to parse chart values: %w", err)
		}
		message.Debug(chartValues)
	} else {
		// Otherwise, use the overrides instead
		loadedChart = options.ChartOverride
		chartValues = options.ValueOverride
	}

	return loadedChart, chartValues, nil
}

func NewRenderer(options ChartOptions) *renderer {
	message.Debugf("helm.NewRenderer(%v)", options)
	return &renderer{
		options:        options,
		namespaces:     []string{options.Chart.Namespace},
		connectStrings: make(ConnectStrings),
	}
}

func (r *renderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	message.Debugf("helm.Run(%v)", renderedManifests)
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	tempDir, _ := utils.MakeTempDir()
	path := tempDir + "/chart.yaml"

	if err := utils.WriteFile(path, renderedManifests.Bytes()); err != nil {
		return nil, fmt.Errorf("unable to write the post-render file for the helm chart")
	}

	// Run the template engine against the chart output
	k8s.ProcessYamlFilesInPath(tempDir, r.options.Images)

	// Read back the final file contents
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading temporary post-rendered helm chart: %w", err)
	}

	message.Debug(string(buff))

	// Try to parse the yaml into unstructured data
	resources, err := k8s.SplitYAML(buff)
	if err != nil {
		// On error only drop a warning
		message.Errorf(err, "Problem parsing post-render manifest data")
	} else {
		// Otherwise, loop over the resources,
		for _, resource := range resources {
			// grab the namespace,
			namespace := resource.GetNamespace()

			// and append to the list if it's unique
			if namespace != "" && !contains(r.namespaces, namespace) {
				r.namespaces = append(r.namespaces, namespace)
			}

			if resource.GetKind() == "Service" {
				// Check service resources for the zarf-connect label
				labels := resource.GetLabels()
				annotations := resource.GetAnnotations()

				if key, keyExists := labels[config.ZarfConnectLabelName]; keyExists {
					// If there is a zarf-connect label
					if description, descExists := annotations[config.ZarfConnectAnnotationDescription]; descExists {
						// and a description set the label and description
						r.connectStrings[key] = description
					} else {
						// Otherwise, just set the label
						r.connectStrings[key] = ""
					}
				}
			}
		}
	}

	chartText := string(buff)
	secretPrefix := "---\n"
	secretName := "zarf-registry"
	for _, namespace := range r.namespaces {
		// Try to get an existing secret
		secret, _ := k8s.GetSecret(namespace, secretName)

		if secret.Name == secretName && secret.Annotations["meta.helm.sh/release-name"] != r.options.ReleaseName {
			// Don't add a secret if it already was created by another chart
			// But we have to include it this chart deployed it or helm will remove it
			continue
		}

		// Create the secret as a k8s object
		secret = k8s.GenerateRegistryPullCreds(namespace, secretName)

		// Convert to yaml buffer
		buf := new(bytes.Buffer)
		yp := printers.YAMLPrinter{}
		yp.PrintObj(secret, buf)

		// Prepend the secret to the helm chart text
		chartText = secretPrefix + buf.String() + chartText

	}

	// Cleanup the temp file
	_ = os.RemoveAll(tempDir)

	// Send the bytes back to helm
	return bytes.NewBuffer([]byte(chartText)), nil
}

func contains(haystack []string, needle string) bool {
	message.Debugf("helm.contains(%v, %s)", haystack, needle)
	for _, hay := range haystack {
		if hay == needle {
			return true
		}
	}
	return false
}
