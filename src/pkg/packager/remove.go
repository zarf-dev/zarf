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
	packageSecret, err := p.cluster.Kube.GetSecret("zarf", secretName)
	if err != nil {
		return fmt.Errorf("unable to get the secret for the package we are attempting to remove: %w", err)
	}

	// Get the list of components the package had deployed
	packages := types.DeployedPackage{}
	err = json.Unmarshal(packageSecret.Data["data"], &packages)
	if err != nil {
		return fmt.Errorf("unable to load the secret for the package we are attempting to remove: %w", err)
	}

	// If components were provided; just remove the things we were asked to remove and return
	requestedComponents := strings.Split(p.cfg.DeployOpts.Components, ",")
	partialRemove := len(requestedComponents) > 0 && requestedComponents[0] != ""

	for _, c := range utils.Reverse(packages.DeployedComponents) {
		// Only remove the component if it was requested or if we are removing the whole package
		if partialRemove && !slices.Contains(requestedComponents, c.Name) {
			continue
		}

		if err := p.removeComponent(packages.Data.Components, c, spinner); err != nil {
			return fmt.Errorf("unable to remove the component (%s): %w", c.Name, err)
		}

		// Remove the component we just removed from the array
		packages.DeployedComponents = utils.RemoveMatches(packages.DeployedComponents, func(t types.DeployedComponent) bool {
			return t.Name == c.Name
		})

		if len(packages.DeployedComponents) < 1 {
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

	return nil
}

func (p *Packager) removeComponent(components []types.ZarfComponent, deployedComponent types.DeployedComponent, spinner *message.Spinner) error {
	c := utils.Find(components, func(t types.ZarfComponent) bool {
		return t.Name == deployedComponent.Name
	})

	onFailure := func() {
		if err := p.runComponentActions(c.Actions.OnRemove, c.Actions.OnRemove.OnFailure); err != nil {
			message.Debugf("Unable to run the failure action: %s", err)
		}
	}

	if err := p.runComponentActions(c.Actions.OnRemove, c.Actions.OnRemove.Before); err != nil {
		onFailure()
		return fmt.Errorf("unable to run the first action for component (%s): %w", c.Name, err)
	}

	for _, chart := range utils.Reverse(deployedComponent.InstalledCharts) {
		spinner.Updatef("Uninstalling chart (%s) from the (%s) component", chart.ChartName, deployedComponent.Name)

		helmCfg := helm.Helm{}
		if err := helmCfg.RemoveChart(chart.Namespace, chart.ChartName, spinner); err != nil {
			onFailure()
			return fmt.Errorf("unable to uninstall the helm chart %s in the namespace %s: %w",
				chart.ChartName, chart.Namespace, err)
		}
	}

	if err := p.runComponentActions(c.Actions.OnRemove, c.Actions.OnRemove.After); err != nil {
		onFailure()
		return fmt.Errorf("unable to run the last action: %w", err)
	}

	if err := p.runComponentActions(c.Actions.OnRemove, c.Actions.OnRemove.OnSuccess); err != nil {
		onFailure()
		return fmt.Errorf("unable to run the success action: %w", err)
	}

	return nil
}
