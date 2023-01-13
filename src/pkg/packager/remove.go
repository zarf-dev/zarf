// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
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

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts.
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
	packageSecret, err := p.cluster.Kube.GetSecret(cluster.ZarfNamespace, secretName)
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

	// If components were provided; just remove the things we were asked to remove
	requestedComponents := strings.Split(p.cfg.DeployOpts.Components, ",")

	// If components were not provided; set things up to remove all package components
	if len(requestedComponents) < 1 || requestedComponents[0] == "" {
		requestedComponents = []string{}

		for _, component := range deployedPackage.DeployedComponents {
			requestedComponents = append(requestedComponents, component.Name)
		}
	}

	// Loop through the deployed components (in reverse order) check if they were requested and remove them if so
	for i := len(deployedPackage.DeployedComponents) - 1; i >= 0; i-- {
		installedComponent := deployedPackage.DeployedComponents[i]

		if slices.Contains(requestedComponents, installedComponent.Name) {
			for h := len(installedComponent.InstalledCharts) - 1; h >= 0; h-- {
				installedChart := installedComponent.InstalledCharts[h]

				spinner.Updatef("Uninstalling chart (%s) from the (%s) component", installedChart.ChartName, installedComponent.Name)

				helmCfg := helm.Helm{}
				err = helmCfg.RemoveChart(installedChart.Namespace, installedChart.ChartName, spinner)
				if err != nil {
					message.Errorf(err, "Unable to remove the installed helm chart (%s) from the namespace (%s) of component (%s) (were dependent components removed first?)",
						installedChart.ChartName, installedChart.Namespace, installedComponent.Name)

					return err
				}

				// Remove the uninstalled chart from the list of installed charts
				deployedPackage.DeployedComponents[i].InstalledCharts = deployedPackage.DeployedComponents[i].InstalledCharts[:h]
				p.updatePackageSecret(deployedPackage, secretName)
			}

			// Remove the component we just removed from the array
			deployedPackage.DeployedComponents = append(deployedPackage.DeployedComponents[:i], deployedPackage.DeployedComponents[i+1:]...)
		}

		if len(deployedPackage.DeployedComponents) == 0 {
			// All the installed components were deleted, there for this package is no longer actually deployed
			_ = p.cluster.Kube.DeleteSecret(packageSecret)
		} else {
			p.updatePackageSecret(deployedPackage, secretName)
		}
	}

	return nil
}

func (p *Packager) updatePackageSecret(deployedPackage types.DeployedPackage, secretName string) {
	// Save the new secret with the removed components removed from the secret
	newPackageSecret := p.cluster.Kube.GenerateSecret(cluster.ZarfNamespace, secretName, corev1.SecretTypeOpaque)
	newPackageSecret.Labels[cluster.ZarfPackageInfoLabel] = p.cfg.Pkg.Metadata.Name

	newPackageSecretData, _ := json.Marshal(deployedPackage)
	newPackageSecret.Data["data"] = newPackageSecretData

	err := p.cluster.Kube.CreateOrUpdateSecret(newPackageSecret)
	if err != nil {
		message.Warnf("Unable to update the %s package secret: %#v", secretName, err)
	}
}
