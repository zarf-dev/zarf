package packager

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/mholt/archiver/v3"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/strings/slices"
)

func Uninstall() {
	// Create temp paths to temporarily extract the package into
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
	installedPackages := zarfState.InstalledPackages
	installedPackage, ok := installedPackages[config.GetActiveConfig().Metadata.Name]
	if !ok {
		message.Fatalf(nil, "We are unable to uninstall %s because it does not appear to have been installed yet", config.DeployOptions.PackagePath)
	}

	// If components were provided; just uninstall the things we were asked to uninstall and return
	requestedComponents := strings.Split(config.DeployOptions.Components, ",")
	if len(requestedComponents) > 0 {
		for componentName, installedComponent := range installedPackage.InstalledComponents {
			if slices.Contains(requestedComponents, componentName) {
				for _, installedChart := range installedComponent.InstalledCharts {
					helm.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
				}

				// Remove the component we just delete from the state
				delete(installedPackage.InstalledComponents, componentName)
			}

			if len(installedPackage.InstalledComponents) == 0 {
				delete(installedPackages, config.GetActiveConfig().Metadata.Name)
			}
		}
	}

	// Go through all the components of the package and prompt if we have that component installed
	nativePackageComponents := config.GetComponents()
	for _, nativeComponent := range nativePackageComponents {
		installedComponent, ok := installedPackage.InstalledComponents[nativeComponent.Name]
		if ok {
			// This component was installed onto the cluster. Prompt the user to see if they would like to uninstall it!
			content, _ := yaml.Marshal(nativeComponent)
			utils.ColorPrintYAML(string(content))
			// TODO: @JPERRY Jeff displayed the description as a question here. maybe I should do that too

			confirmUninstall := false
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Uninstall the %s component?", nativeComponent.Name),
				Default: false,
			}
			if err := survey.AskOne(prompt, &confirmUninstall); err != nil {
				message.Fatalf(err, "Confirm selection canceled")
			}

			if confirmUninstall {
				for _, installedChart := range installedComponent.InstalledCharts {
					fmt.Printf("Uninstalling chart (%s) from the (%s) component", installedChart.ChartName, nativeComponent.Name)
					helm.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
				}
			}
		}
	}
}
