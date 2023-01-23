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
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/strings/slices"
)

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts.
func (p *Packager) Remove(packageName string) (err error) {
	spinner := message.NewProgressSpinner("Removing zarf package %s", packageName)
	defer spinner.Stop()

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
		return fmt.Errorf("unable to get the secret for the package we are attempting to remove: %w", err)
	}

	// Get the list of components the package had deployed
	deployedPackage := types.DeployedPackage{}
	err = json.Unmarshal(packageSecret.Data["data"], &deployedPackage)
	if err != nil {
		return fmt.Errorf("unable to load the secret for the package we are attempting to remove: %w", err)
	}

	// If components were provided; just remove the things we were asked to remove
	requestedComponents := strings.Split(p.cfg.DeployOpts.Components, ",")
	partialRemove := len(requestedComponents) > 0 && requestedComponents[0] != ""

	for _, c := range utils.Reverse(deployedPackage.DeployedComponents) {
		// Only remove the component if it was requested or if we are removing the whole package
		if partialRemove && !slices.Contains(requestedComponents, c.Name) {
			continue
		}

		if deployedPackage, err = p.removeComponent(deployedPackage, c, secretName, spinner); err != nil {
			return fmt.Errorf("unable to remove the component (%s): %w", c.Name, err)
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

func (p *Packager) removeComponent(deployedPackage types.DeployedPackage, deployedComponent types.DeployedComponent, secretName string, spinner *message.Spinner) (types.DeployedPackage, error) {
	components := deployedPackage.Data.Components

	c := utils.Find(components, func(t types.ZarfComponent) bool {
		return t.Name == deployedComponent.Name
	})

	onRemove := c.Actions.OnRemove
	onFailure := func() {
		if err := p.runActions(onRemove.Defaults, onRemove.OnFailure, nil); err != nil {
			message.Debugf("Unable to run the failure action: %s", err)
		}
	}

	if err := p.runActions(onRemove.Defaults, onRemove.Before, nil); err != nil {
		onFailure()
		return deployedPackage, fmt.Errorf("unable to run the before action for component (%s): %w", c.Name, err)
	}

	for _, chart := range utils.Reverse(deployedComponent.InstalledCharts) {
		spinner.Updatef("Uninstalling chart (%s) from the (%s) component", chart.ChartName, deployedComponent.Name)

		helmCfg := helm.Helm{}
		if err := helmCfg.RemoveChart(chart.Namespace, chart.ChartName, spinner); err != nil {
			onFailure()
			return deployedPackage, fmt.Errorf("unable to uninstall the helm chart %s in the namespace %s: %w",
				chart.ChartName, chart.Namespace, err)
		}

		// Remove the uninstalled chart from the list of installed charts
		// NOTE: We are saving the secret as we remove charts in case a failure happens later on in the process of removing the component.
		//       If we don't save the secrets as we remove charts, we will run into issues if we try to remove the component again as we will
		//       be trying to remove charts that have already been removed.
		deployedComponent.InstalledCharts = utils.RemoveMatches(deployedComponent.InstalledCharts, func(t types.InstalledChart) bool {
			return t.ChartName == chart.ChartName
		})
		p.updatePackageSecret(deployedPackage, secretName)

	}

	if err := p.runActions(onRemove.Defaults, onRemove.After, nil); err != nil {
		onFailure()
		return deployedPackage, fmt.Errorf("unable to run the after action: %w", err)
	}

	if err := p.runActions(onRemove.Defaults, onRemove.OnSuccess, nil); err != nil {
		onFailure()
		return deployedPackage, fmt.Errorf("unable to run the success action: %w", err)
	}

	// Remove the component we just removed from the array
	deployedPackage.DeployedComponents = utils.RemoveMatches(deployedPackage.DeployedComponents, func(t types.DeployedComponent) bool {
		return t.Name == c.Name
	})

	if len(deployedPackage.DeployedComponents) == 0 {
		// All the installed components were deleted, therefore this package is no longer actually deployed
		packageSecret, err := p.cluster.Kube.GetSecret(cluster.ZarfNamespace, secretName)
		if err != nil {
			return deployedPackage, fmt.Errorf("unable to get the secret for the package we are attempting to remove: %w", err)
		}
		_ = p.cluster.Kube.DeleteSecret(packageSecret)
	} else {
		p.updatePackageSecret(deployedPackage, secretName)
	}

	return deployedPackage, nil
}
