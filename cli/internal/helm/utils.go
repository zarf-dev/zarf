package helm

import (
	"fmt"
	"github.com/defenseunicorns/zarf/cli/types"
	"os"
	"strconv"

	"github.com/defenseunicorns/zarf/cli/internal/message"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"

	"helm.sh/helm/v3/pkg/chart/loader"
)

// StandardName generates a predictable full path for a helm chart for Zarf
func StandardName(destination string, chart types.ZarfChart) string {
	return destination + "/" + chart.Name + "-" + chart.Version
}

// loadChartFromTarball returns a helm chart from a tarball
func loadChartFromTarball(options ChartOptions) (*chart.Chart, error) {
	// Get the path the temporary helm chart tarball
	sourceTarball := StandardName(options.BasePath+"/charts", options.Chart) + ".tgz"

	// Load the loadedChart tarball
	loadedChart, err := loader.Load(sourceTarball)
	if err != nil {
		return nil, fmt.Errorf("unable to load helm chart archive: %w", err)
	}

	if err = loadedChart.Validate(); err != nil {
		return nil, fmt.Errorf("unable to validate loaded helm chart: %w", err)
	}

	return loadedChart, nil
}

// parseChartValues reads the context of the chart values into an interface if it exists
func parseChartValues(options ChartOptions) (map[string]interface{}, error) {
	valueOpts := &values.Options{}

	for idx := range options.Chart.ValuesFiles {
		path := StandardName(options.BasePath+"/values", options.Chart) + "-" + strconv.Itoa(idx)
		valueOpts.ValueFiles = append(valueOpts.ValueFiles, path)
	}

	httpProvider := getter.Provider{
		Schemes: []string{"http", "https"},
		New:     getter.NewHTTPGetter,
	}

	providers := getter.Providers{httpProvider}
	return valueOpts.MergeValues(providers)
}

func createActionConfig(namespace string) (*action.Configuration, error) {
	// OMG THIS IS SOOOO GROSS PPL... https://github.com/helm/helm/issues/8780
	_ = os.Setenv("HELM_NAMESPACE", namespace)

	// Initialize helm SDK
	actionConfig := new(action.Configuration)
	settings := cli.New()

	// Setup K8s connection
	err := actionConfig.Init(settings.RESTClientGetter(), namespace, "", message.Debugf)

	return actionConfig, err
}
