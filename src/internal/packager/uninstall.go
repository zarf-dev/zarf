package packager

import (
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/mholt/archiver/v3"
)

func Uninstall() {
	tempPath := createPaths()
	defer tempPath.clean()

	spinner := message.NewProgressSpinner("Preparing zarf package %s", config.DeployOptions.PackagePath)
	defer spinner.Stop()

	// Extract the archive
	spinner.Updatef("Extracting the package, this may take a few moments")
	err := archiver.Unarchive(config.DeployOptions.PackagePath, tempPath.base)
	if err != nil {
		spinner.Fatalf(err, "Unable to extract the package contents")
	}

	// Load the config from the extracted archive zarf.yaml
	spinner.Updatef("Loading the zarf package config")
	configPath := filepath.Join(tempPath.base, "zarf.yaml")
	if err := config.LoadConfig(configPath, false); err != nil {
		spinner.Fatalf(err, "Invalid or unreadable zarf.yaml file in %s", tempPath.base)
	}

	// Get the list of installed packages/charts from the state
	zarfState := k8s.LoadZarfState()
	installedPackage, ok := zarfState.InstalledPackages[config.GetActiveConfig().Metadata.Name]
	if !ok {
		message.Fatalf(nil, "We are unable to uninstall %s because it does not appear to have been installed yet", config.DeployOptions.PackagePath)
	}

	// Actually install the things
	for componentName, installedComponent := range installedPackage.InstalledComponents {
		message.Notef("Uninstalling the charts for the component: %s", componentName)
		// for

		for _, installedChart := range installedComponent.InstalledCharts {
			fmt.Printf("Uninstalling chart (%s) from the (%s) component", installedChart.ChartName, componentName)
			helm.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
		}
	}
}
