// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/zarf-dev/zarf/src/pkg/logger"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/storage/driver"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/types"
)

// RemoveOptions are the options for Remove.
type RemoveOptions struct {
	Source                  string
	Cluster                 *cluster.Cluster
	Filter                  filters.ComponentFilterStrategy
	SkipSignatureValidation bool
	PublicKeyPath           string
}

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts.
func Remove(ctx context.Context, opt RemoveOptions) error {
	l := logger.From(ctx)
	pkg, err := GetPackageFromSourceOrCluster(ctx, opt.Cluster, opt.Source, opt.SkipSignatureValidation, opt.PublicKeyPath)
	if err != nil {
		return err
	}

	// If components were provided; just remove the things we were asked to remove
	components, err := opt.Filter.Apply(pkg)
	if err != nil {
		return err
	}
	// Check that cluster is configured if required.
	requiresCluster := false
	componentIdx := map[string]v1alpha1.ZarfComponent{}
	for _, component := range components {
		componentIdx[component.Name] = component
		if component.RequiresCluster() {
			if opt.Cluster == nil {
				return fmt.Errorf("component %s requires cluster access but none was configured", component.Name)
			}
			requiresCluster = true
		}
	}

	// Get or build the secret for the deployed package
	depPkg := &types.DeployedPackage{}
	if requiresCluster {
		depPkg, err = opt.Cluster.GetDeployedPackage(ctx, pkg.Metadata.Name)
		if err != nil {
			return fmt.Errorf("unable to load the secret for the package we are attempting to remove: %s", err.Error())
		}
	} else {
		// If we do not need the cluster, create a deployed components object based on the info we have
		depPkg.Name = pkg.Metadata.Name
		depPkg.Data = pkg
		for _, component := range components {
			depPkg.DeployedComponents = append(depPkg.DeployedComponents, types.DeployedComponent{Name: component.Name})
		}
	}

	reverseDepComps := slices.Clone(depPkg.DeployedComponents)
	slices.Reverse(reverseDepComps)
	for _, depComp := range reverseDepComps {
		// Only remove the component if it was requested or if we are removing the whole package.
		comp, ok := componentIdx[depComp.Name]
		if !ok {
			continue
		}

		err := func() error {
			err := actions.Run(ctx, comp.Actions.OnRemove.Defaults, comp.Actions.OnRemove.Before, nil)
			if err != nil {
				return fmt.Errorf("unable to run the before action: %w", err)
			}

			reverseInstalledCharts := slices.Clone(depComp.InstalledCharts)
			slices.Reverse(reverseInstalledCharts)
			if opt.Cluster != nil {
				for _, chart := range reverseInstalledCharts {
					settings := cli.New()
					settings.SetNamespace(chart.Namespace)
					actionConfig := &action.Configuration{}
					// TODO (phillebaba): Get credentials from cluster instead of reading again.
					err := actionConfig.Init(settings.RESTClientGetter(), chart.Namespace, "", func(string, ...interface{}) {})
					if err != nil {
						return err
					}
					client := action.NewUninstall(actionConfig)
					client.KeepHistory = false
					client.Wait = true
					client.Timeout = config.ZarfDefaultTimeout
					_, err = client.Run(chart.ChartName)
					if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
						return fmt.Errorf("unable to uninstall the helm chart %s in the namespace %s: %w", chart.ChartName, chart.Namespace, err)
					}
					if errors.Is(err, driver.ErrReleaseNotFound) {
						l.Warn("helm release was not found. was it already removed?", "name", chart.ChartName, "namespace", chart.Namespace)
					}

					// Pop the removed helm chart from the installed charts slice.
					installedCharts := depPkg.DeployedComponents[len(depPkg.DeployedComponents)-1].InstalledCharts
					installedCharts = installedCharts[:len(installedCharts)-1]
					depPkg.DeployedComponents[len(depPkg.DeployedComponents)-1].InstalledCharts = installedCharts
					err = opt.Cluster.UpdateDeployedPackage(ctx, *depPkg)
					if err != nil {
						// We warn and ignore errors because we may have removed the cluster that this package was inside of
						l.Warn("unable to update secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
					}
				}
			}

			err = actions.Run(ctx, comp.Actions.OnRemove.Defaults, comp.Actions.OnRemove.After, nil)
			if err != nil {
				return fmt.Errorf("unable to run the after action: %w", err)
			}
			err = actions.Run(ctx, comp.Actions.OnRemove.Defaults, comp.Actions.OnRemove.OnSuccess, nil)
			if err != nil {
				return fmt.Errorf("unable to run the success action: %w", err)
			}

			// Pop the removed component from deploy components slice.
			if opt.Cluster != nil {
				depPkg.DeployedComponents = depPkg.DeployedComponents[:len(depPkg.DeployedComponents)-1]
				err = opt.Cluster.UpdateDeployedPackage(ctx, *depPkg)
				if err != nil {
					// We warn and ignore errors because we may have removed the cluster that this package was inside of
					l.Warn("unable to update secret package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
				}
			}
			return nil
		}()
		if err != nil {
			removeErr := actions.Run(ctx, comp.Actions.OnRemove.Defaults, comp.Actions.OnRemove.OnFailure, nil)
			if removeErr != nil {
				return errors.Join(fmt.Errorf("unable to run the failure action: %w", err), removeErr)
			}
			return err
		}
	}

	// All the installed components were deleted, therefore this package is no longer actually deployed
	if opt.Cluster != nil && len(depPkg.DeployedComponents) == 0 {
		err := opt.Cluster.DeleteDeployedPackage(ctx, depPkg.Name)
		if err != nil {
			l.Warn("unable to delete secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
		}
	}

	l.Info("package successfully removed", "name", pkg.Metadata.Name)
	return nil
}
