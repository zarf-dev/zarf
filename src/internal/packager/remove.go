package packager

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/helm"
	"github.com/defenseunicorns/zarf/src/internal/k8s"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/types"
	"k8s.io/utils/strings/slices"
)

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts
func Remove(packageName string) {
	// Create temp paths to temporarily extract the package into
	tempPath := createPaths()
	defer tempPath.clean()

	spinner := message.NewProgressSpinner("Removing zarf package %s", packageName)
	defer spinner.Stop()

	// Get the secret for the deployed package
	secretName := fmt.Sprintf("zarf-package-%s", packageName)
	packageSecret, err := k8s.GetSecret("zarf", secretName)
	if err != nil {
		spinner.Fatalf(err, "Unable to get the secret for the package we are attempting to remove")
	}

	// Get the list of components the package had deployed
	deployedPackage := types.DeployedPackage{}
	err = json.Unmarshal(packageSecret.Data["data"], &deployedPackage)
	if err != nil {
		spinner.Fatalf(err, "Unable to load the secret for the package we are attempting to remove")
	}

	// If components were provided; just remove the things we were asked to remove and return
	requestedComponents := strings.Split(config.DeployOptions.Components, ",")
	if len(requestedComponents) > 0 && requestedComponents[0] != "" {
		for componentName, installedComponent := range deployedPackage.DeployedComponents {
			if slices.Contains(requestedComponents, componentName) {
				for _, installedChart := range installedComponent.InstalledCharts {
					helm.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
				}

				// Remove the component we just removed from the map
				delete(deployedPackage.DeployedComponents, componentName)
			}

			if len(deployedPackage.DeployedComponents) == 0 {
				// All the installed components were deleted, there for this package is no longer actually deployed
				_ = k8s.DeleteSecret(packageSecret)
			} else {
				// Save the new secret with the removed components removed from the secret
				newPackageSecretData, _ := json.Marshal(deployedPackage)
				packageSecret.Data["data"] = newPackageSecretData
				_ = k8s.ReplaceSecret(packageSecret)
			}
		}
	} else {
		// Loop through all the installed components and remove them
		for componentName, nativeComponent := range deployedPackage.DeployedComponents {
			// This component was installed onto the cluster. Prompt the user to see if they would like to remove it!
			for _, installedChart := range nativeComponent.InstalledCharts {
				spinner.Updatef("Uninstalling chart (%s) from the (%s) component", installedChart.ChartName, componentName)
				_ = helm.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
			}
		}
		k8s.DeleteSecret(packageSecret)
	}
}
