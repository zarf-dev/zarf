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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/strings/slices"
)

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts
func Remove(packageName string) error {
	// Create temp paths to temporarily extract the package into
	tempPath := createPaths()
	defer tempPath.clean()

	spinner := message.NewProgressSpinner("Removing zarf package %s", packageName)
	defer spinner.Stop()

	// Get the secret for the deployed package
	secretName := fmt.Sprintf("zarf-package-%s", packageName)
	packageSecret, err := k8s.GetSecret("zarf", secretName)
	if err != nil {
		spinner.Errorf(err, "Unable to get the secret for the package we are attempting to remove")

		return err
	}

	// Get the list of components the package had deployed
	deployedPackage := types.DeployedPackage{}
	err = json.Unmarshal(packageSecret.Data["data"], &deployedPackage)
	if err != nil {
		spinner.Errorf(err, "Unable to load the secret for the package we are attempting to remove")

		return err
	}

	// If components were provided; just remove the things we were asked to remove and return
	requestedComponents := strings.Split(config.DeployOptions.Components, ",")
	if len(requestedComponents) > 0 && requestedComponents[0] != "" {
		for i := len(deployedPackage.DeployedComponents) - 1; i >= 0; i-- {
			installedComponent := deployedPackage.DeployedComponents[i]

			if slices.Contains(requestedComponents, installedComponent.Name) {
				for _, installedChart := range installedComponent.InstalledCharts {
					helm.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
				}

				// Remove the component we just removed from the array
				deployedPackage.DeployedComponents = append(deployedPackage.DeployedComponents[:i], deployedPackage.DeployedComponents[i+1:]...)
			}

			if len(deployedPackage.DeployedComponents) == 0 {
				// All the installed components were deleted, there for this package is no longer actually deployed
				_ = k8s.DeleteSecret(packageSecret)
			} else {
				// Save the new secret with the removed components removed from the secret
				newPackageSecret := k8s.GenerateSecret("zarf", secretName, corev1.SecretTypeOpaque)
				newPackageSecret.Labels["package-deploy-info"] = config.GetActiveConfig().Metadata.Name
				newPackageSecretData, _ := json.Marshal(deployedPackage)
				newPackageSecret.Data["data"] = newPackageSecretData
				err = k8s.ReplaceSecret(newPackageSecret)
				if err != nil {
					message.Warnf("Unable to replace the %s package secret: %#v", secretName, err)
				}
			}
		}
	} else {
		// Loop through all the installed components and remove them
		for i := len(deployedPackage.DeployedComponents) - 1; i >= 0; i-- {
			installedComponent := deployedPackage.DeployedComponents[i]

			// This component was installed onto the cluster. Prompt the user to see if they would like to remove it!
			for _, installedChart := range installedComponent.InstalledCharts {
				spinner.Updatef("Uninstalling chart (%s) from the (%s) component", installedChart.ChartName, installedComponent.Name)

				err = helm.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
				if err != nil {
					message.Errorf(err, "Unable to remove the installed helm chart (%s) from the namespace (%s) of component (%s) (were dependent components removed first?)",
						installedChart.ChartName, installedChart.Namespace, installedComponent.Name)

					return err
				}
			}
		}
		k8s.DeleteSecret(packageSecret)
	}

	return nil
}
