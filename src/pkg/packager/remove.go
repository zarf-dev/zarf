// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
package packager

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/strings/slices"
)

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts
func (p *Packager) Remove(packageName string) error {
	spinner := message.NewProgressSpinner("Removing zarf package %s", packageName)
	defer spinner.Stop()

	var err error
	if p.cluster == nil {
		p.cluster, err = cluster.NewClusterWithWait(30 * time.Second)
		if err != nil {
			return fmt.Errorf("unable to connect to the Kubernetes cluster: %w", err)
		}
	}

	// Get the secret for the deployed package
	secretName := config.ZarfPackagePrefix + packageName
	packageSecret, err := p.cluster.Kube.GetSecret("zarf", secretName)
	if err != nil {
		spinner.Errorf(err, "Unable to get the secret for the package we are attempting to remove")

		return err
	}

	// Get the list of components the package had deployed
	packages := types.DeployedPackage{}
	err = json.Unmarshal(packageSecret.Data["data"], &packages)
	if err != nil {
		spinner.Errorf(err, "Unable to load the secret for the package we are attempting to remove")

		return err
	}

	// If components were provided; just remove the things we were asked to remove and return
	requestedComponents := strings.Split(p.cfg.DeployOpts.Components, ",")
	if len(requestedComponents) > 0 && requestedComponents[0] != "" {
		for i := len(packages.DeployedComponents) - 1; i >= 0; i-- {
			installedComponent := packages.DeployedComponents[i]

			if slices.Contains(requestedComponents, installedComponent.Name) {
				for _, installedChart := range installedComponent.InstalledCharts {
					helmCfg := helm.Helm{}
					helmCfg.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
				}

				// Remove the component we just removed from the array
				packages.DeployedComponents = append(packages.DeployedComponents[:i], packages.DeployedComponents[i+1:]...)
			}

			if len(packages.DeployedComponents) == 0 {
				// All the installed components were deleted, there for this package is no longer actually deployed
				_ = p.cluster.Kube.DeleteSecret(packageSecret)
			} else {
				// Save the new secret with the removed components removed from the secret
				newPackageSecret := p.cluster.Kube.GenerateSecret("zarf", secretName, corev1.SecretTypeOpaque)
				newPackageSecret.Labels["package-deploy-info"] = p.cfg.Pkg.Metadata.Name
				newPackageSecretData, _ := json.Marshal(packages)
				newPackageSecret.Data["data"] = newPackageSecretData
				err = p.cluster.Kube.ReplaceSecret(newPackageSecret)
				if err != nil {
					message.Warnf("Unable to replace the %s package secret: %#v", secretName, err)
				}
			}
		}
	} else {
		// Loop through all the installed components and remove them
		for i := len(packages.DeployedComponents) - 1; i >= 0; i-- {
			installedComponent := packages.DeployedComponents[i]

			// This component was installed onto the cluster. Prompt the user to see if they would like to remove it!
			for _, installedChart := range installedComponent.InstalledCharts {
				spinner.Updatef("Uninstalling chart (%s) from the (%s) component", installedChart.ChartName, installedComponent.Name)

				helmCfg := helm.Helm{}
				err = helmCfg.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
				if err != nil {
					message.Errorf(err, "Unable to remove the installed helm chart (%s) from the namespace (%s) of component (%s) (were dependent components removed first?)",
						installedChart.ChartName, installedChart.Namespace, installedComponent.Name)

					return err
				}
			}
		}
		p.cluster.Kube.DeleteSecret(packageSecret)
	}

	return nil
}
