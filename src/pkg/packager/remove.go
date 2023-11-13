// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"encoding/json"
	"errors"
	"fmt"

	"slices"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/helm"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
)

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts.
func (p *Packager) Remove() (err error) {
	_, requiresCluster := p.source.(*sources.ClusterSource)
	if requiresCluster {
		p.cluster = p.source.(*sources.ClusterSource).Cluster
	}
	spinner := message.NewProgressSpinner("Removing Zarf package %s", p.cfg.PkgOpts.PackageSource)
	defer spinner.Stop()

	// If components were provided; just remove the things we were asked to remove
	requestedComponents := helpers.StringToSlice(p.cfg.PkgOpts.OptionalComponents)
	partialRemove := len(requestedComponents) > 0 && requestedComponents[0] != ""

	var packageName string

	// we do not want to allow removal of signed packages without a signature if there are remove actions
	// as this is arbitrary code execution from an untrusted source
	if err = p.source.LoadPackageMetadata(p.layout, false, false); err != nil {
		return err
	}
	if err = p.readZarfYAML(p.layout.ZarfYAML); err != nil {
		return err
	}
	p.filterComponents()
	packageName = p.cfg.Pkg.Metadata.Name

	// If we have package components check them for images, charts, manifests, etc
	for _, component := range p.cfg.Pkg.Components {
		// Flip requested based on if this is a partial removal
		requested := !partialRemove

		if slices.Contains(requestedComponents, component.Name) {
			requested = true
		}

		if requested {
			if p.requiresCluster(component) {
				requiresCluster = true
			}
		}
	}

	// Get the secret for the deployed package
	deployedPackage := &types.DeployedPackage{}

	if requiresCluster {
		err = p.connectToCluster(cluster.DefaultTimeout)
		if err != nil {
			return err
		}
		deployedPackage, err = p.cluster.GetDeployedPackage(packageName)
		if err != nil {
			return fmt.Errorf("unable to load the secret for the package we are attempting to remove: %s", err.Error())
		}
	} else {
		// If we do not need the cluster, create a deployed components object based on the info we have
		deployedPackage.Name = packageName
		deployedPackage.Data = p.cfg.Pkg
		if partialRemove {
			for _, r := range requestedComponents {
				deployedPackage.DeployedComponents = append(deployedPackage.DeployedComponents, types.DeployedComponent{Name: r})
			}
		} else {
			for _, c := range p.cfg.Pkg.Components {
				deployedPackage.DeployedComponents = append(deployedPackage.DeployedComponents, types.DeployedComponent{Name: c.Name})
			}
		}
	}

	for _, c := range helpers.Reverse(deployedPackage.DeployedComponents) {
		// Only remove the component if it was requested or if we are removing the whole package
		if partialRemove && !slices.Contains(requestedComponents, c.Name) {
			continue
		}

		if deployedPackage, err = p.removeComponent(deployedPackage, c, spinner); err != nil {
			return fmt.Errorf("unable to remove the component '%s': %w", c.Name, err)
		}
	}

	return nil
}

func (p *Packager) updatePackageSecret(deployedPackage types.DeployedPackage) {
	// Only attempt to update the package secret if we are actually connected to a cluster
	if p.cluster != nil {
		secretName := config.ZarfPackagePrefix + deployedPackage.Name

		// Save the new secret with the removed components removed from the secret
		newPackageSecret := p.cluster.GenerateSecret(cluster.ZarfNamespaceName, secretName, corev1.SecretTypeOpaque)
		newPackageSecret.Labels[cluster.ZarfPackageInfoLabel] = deployedPackage.Name

		newPackageSecretData, _ := json.Marshal(deployedPackage)
		newPackageSecret.Data["data"] = newPackageSecretData

		_, err := p.cluster.CreateOrUpdateSecret(newPackageSecret)

		// We warn and ignore errors because we may have removed the cluster that this package was inside of
		if err != nil {
			message.Warnf("Unable to update the '%s' package secret: '%s' (this may be normal if the cluster was removed)", secretName, err.Error())
		}
	}
}

func (p *Packager) removeComponent(deployedPackage *types.DeployedPackage, deployedComponent types.DeployedComponent, spinner *message.Spinner) (*types.DeployedPackage, error) {
	components := deployedPackage.Data.Components

	c := helpers.Find(components, func(t types.ZarfComponent) bool {
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
		return nil, fmt.Errorf("unable to run the before action for component (%s): %w", c.Name, err)
	}

	for _, chart := range helpers.Reverse(deployedComponent.InstalledCharts) {
		spinner.Updatef("Uninstalling chart '%s' from the '%s' component", chart.ChartName, deployedComponent.Name)

		helmCfg := helm.Helm{}
		if err := helmCfg.RemoveChart(chart.Namespace, chart.ChartName, spinner); err != nil {
			if !errors.Is(err, driver.ErrReleaseNotFound) {
				onFailure()
				return deployedPackage, fmt.Errorf("unable to uninstall the helm chart %s in the namespace %s: %w",
					chart.ChartName, chart.Namespace, err)
			}
			message.Warnf("Helm release for helm chart '%s' in the namespace '%s' was not found.  Was it already removed?",
				chart.ChartName, chart.Namespace)
		}

		// Remove the uninstalled chart from the list of installed charts
		// NOTE: We are saving the secret as we remove charts in case a failure happens later on in the process of removing the component.
		//       If we don't save the secrets as we remove charts, we will run into issues if we try to remove the component again as we will
		//       be trying to remove charts that have already been removed.
		deployedComponent.InstalledCharts = helpers.RemoveMatches(deployedComponent.InstalledCharts, func(t types.InstalledChart) bool {
			return t.ChartName == chart.ChartName
		})
		p.updatePackageSecret(*deployedPackage)
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
	deployedPackage.DeployedComponents = helpers.RemoveMatches(deployedPackage.DeployedComponents, func(t types.DeployedComponent) bool {
		return t.Name == c.Name
	})

	if len(deployedPackage.DeployedComponents) == 0 && p.cluster != nil {
		secretName := config.ZarfPackagePrefix + deployedPackage.Name

		// All the installed components were deleted, therefore this package is no longer actually deployed
		packageSecret, err := p.cluster.GetSecret(cluster.ZarfNamespaceName, secretName)

		// We warn and ignore errors because we may have removed the cluster that this package was inside of
		if err != nil {
			message.Warnf("Unable to delete the '%s' package secret: '%s' (this may be normal if the cluster was removed)", secretName, err.Error())
		} else {
			err = p.cluster.DeleteSecret(packageSecret)
			if err != nil {
				message.Warnf("Unable to delete the '%s' package secret: '%s' (this may be normal if the cluster was removed)", secretName, err.Error())
			}
		}
	} else {
		p.updatePackageSecret(*deployedPackage)
	}

	return deployedPackage, nil
}
