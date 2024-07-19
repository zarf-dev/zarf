// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
	"github.com/zarf-dev/zarf/src/types"
)

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts.
func (p *Packager) Remove(ctx context.Context) (err error) {
	_, isClusterSource := p.source.(*sources.ClusterSource)
	if isClusterSource {
		p.cluster = p.source.(*sources.ClusterSource).Cluster
	}
	spinner := message.NewProgressSpinner("Removing Zarf package %s", p.cfg.PkgOpts.PackageSource)
	defer spinner.Stop()

	// we do not want to allow removal of signed packages without a signature if there are remove actions
	// as this is arbitrary code execution from an untrusted source
	p.cfg.Pkg, _, err = p.source.LoadPackageMetadata(ctx, p.layout, false, false)
	if err != nil {
		return err
	}
	packageName := p.cfg.Pkg.Metadata.Name

	// Build a list of components to remove and determine if we need a cluster connection
	componentsToRemove := []string{}
	packageRequiresCluster := false

	// If components were provided; just remove the things we were asked to remove
	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.BySelectState(p.cfg.PkgOpts.OptionalComponents),
	)
	included, err := filter.Apply(p.cfg.Pkg)
	if err != nil {
		return err
	}

	for _, component := range included {
		componentsToRemove = append(componentsToRemove, component.Name)

		if component.RequiresCluster() {
			packageRequiresCluster = true
		}
	}

	// Get or build the secret for the deployed package
	deployedPackage := &types.DeployedPackage{}

	if packageRequiresCluster {
		connectCtx, cancel := context.WithTimeout(ctx, cluster.DefaultTimeout)
		defer cancel()
		err = p.connectToCluster(connectCtx)
		if err != nil {
			return err
		}
		deployedPackage, err = p.cluster.GetDeployedPackage(ctx, packageName)
		if err != nil {
			return fmt.Errorf("unable to load the secret for the package we are attempting to remove: %s", err.Error())
		}
	} else {
		// If we do not need the cluster, create a deployed components object based on the info we have
		deployedPackage.Name = packageName
		deployedPackage.Data = p.cfg.Pkg
		for _, r := range componentsToRemove {
			deployedPackage.DeployedComponents = append(deployedPackage.DeployedComponents, types.DeployedComponent{Name: r})
		}
	}

	for _, dc := range helpers.Reverse(deployedPackage.DeployedComponents) {
		// Only remove the component if it was requested or if we are removing the whole package
		if !slices.Contains(componentsToRemove, dc.Name) {
			continue
		}

		if deployedPackage, err = p.removeComponent(ctx, deployedPackage, dc, spinner); err != nil {
			return fmt.Errorf("unable to remove the component '%s': %w", dc.Name, err)
		}
	}

	return nil
}

func (p *Packager) updatePackageSecret(ctx context.Context, deployedPackage types.DeployedPackage) error {
	// Only attempt to update the package secret if we are actually connected to a cluster
	if p.cluster != nil {
		newPackageSecretData, err := json.Marshal(deployedPackage)
		if err != nil {
			return err
		}

		secretName := config.ZarfPackagePrefix + deployedPackage.Name

		// Save the new secret with the removed components removed from the secret
		newPackageSecret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: cluster.ZarfNamespaceName,
				Labels: map[string]string{
					cluster.ZarfManagedByLabel:   "zarf",
					cluster.ZarfPackageInfoLabel: deployedPackage.Name,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"data": newPackageSecretData,
			},
		}

		err = func() error {
			_, err := p.cluster.Clientset.CoreV1().Secrets(newPackageSecret.Namespace).Get(ctx, newPackageSecret.Name, metav1.GetOptions{})
			if err != nil && !kerrors.IsNotFound(err) {
				return err
			}
			if kerrors.IsNotFound(err) {
				_, err = p.cluster.Clientset.CoreV1().Secrets(newPackageSecret.Namespace).Create(ctx, newPackageSecret, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("unable to create the zarf state secret: %w", err)
				}
				return nil
			}
			_, err = p.cluster.Clientset.CoreV1().Secrets(newPackageSecret.Namespace).Update(ctx, newPackageSecret, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("unable to update the zarf state secret: %w", err)
			}
			return nil
		}()
		// We warn and ignore errors because we may have removed the cluster that this package was inside of
		if err != nil {
			message.Warnf("Unable to update the '%s' package secret: '%s' (this may be normal if the cluster was removed)", secretName, err.Error())
		}
	}
	return nil
}

func (p *Packager) removeComponent(ctx context.Context, deployedPackage *types.DeployedPackage, deployedComponent types.DeployedComponent, spinner *message.Spinner) (*types.DeployedPackage, error) {
	components := deployedPackage.Data.Components

	c := helpers.Find(components, func(t types.ZarfComponent) bool {
		return t.Name == deployedComponent.Name
	})

	onRemove := c.Actions.OnRemove
	onFailure := func() {
		if err := actions.Run(onRemove.Defaults, onRemove.OnFailure, nil); err != nil {
			message.Debugf("Unable to run the failure action: %s", err)
		}
	}

	if err := actions.Run(onRemove.Defaults, onRemove.Before, nil); err != nil {
		onFailure()
		return nil, fmt.Errorf("unable to run the before action for component (%s): %w", c.Name, err)
	}

	for _, chart := range helpers.Reverse(deployedComponent.InstalledCharts) {
		spinner.Updatef("Uninstalling chart '%s' from the '%s' component", chart.ChartName, deployedComponent.Name)

		helmCfg := helm.NewClusterOnly(p.cfg, p.variableConfig, p.state, p.cluster)
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
		err := p.updatePackageSecret(ctx, *deployedPackage)
		if err != nil {
			return nil, err
		}
	}

	if err := actions.Run(onRemove.Defaults, onRemove.After, nil); err != nil {
		onFailure()
		return deployedPackage, fmt.Errorf("unable to run the after action: %w", err)
	}

	if err := actions.Run(onRemove.Defaults, onRemove.OnSuccess, nil); err != nil {
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
		packageSecret, err := p.cluster.Clientset.CoreV1().Secrets(cluster.ZarfNamespaceName).Get(ctx, secretName, metav1.GetOptions{})

		// We warn and ignore errors because we may have removed the cluster that this package was inside of
		if err != nil {
			message.Warnf("Unable to delete the '%s' package secret: '%s' (this may be normal if the cluster was removed)", secretName, err.Error())
		} else {
			err = p.cluster.Clientset.CoreV1().Secrets(packageSecret.Namespace).Delete(ctx, packageSecret.Name, metav1.DeleteOptions{})
			if err != nil {
				message.Warnf("Unable to delete the '%s' package secret: '%s' (this may be normal if the cluster was removed)", secretName, err.Error())
			}
		}
	} else {
		err := p.updatePackageSecret(ctx, *deployedPackage)
		if err != nil {
			return nil, err
		}
	}

	return deployedPackage, nil
}
