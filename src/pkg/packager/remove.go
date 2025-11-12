// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"slices"
	"time"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/internal/feature"
	"github.com/zarf-dev/zarf/src/internal/packager/requirements"
	"github.com/zarf-dev/zarf/src/internal/value"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/storage/driver"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

// RemoveOptions are the options for Remove.
type RemoveOptions struct {
	Cluster           *cluster.Cluster
	Timeout           time.Duration
	NamespaceOverride string
	SkipVersionCheck  bool
	// Values passed in at remove time. They can come from the CLI or set directly by API callers.
	value.Values
}

// Remove removes a package that was already deployed onto a cluster, uninstalling all installed helm charts.
func Remove(ctx context.Context, pkg v1alpha1.ZarfPackage, opts RemoveOptions) error {
	l := logger.From(ctx)

	// Validate operational requirements before proceeding
	if !opts.SkipVersionCheck {
		if err := requirements.ValidateVersionRequirements(pkg); err != nil {
			return fmt.Errorf("%w If you cannot upgrade Zarf you may skip this check with --skip-version-check. Unexpected behavior or errors may occur", err)
		}
	}

	var err error
	pkg.Components, err = filters.ByLocalOS(runtime.GOOS).Apply(pkg)
	if err != nil {
		return err
	}

	if len(pkg.Components) == 0 {
		return fmt.Errorf("package to remove contains no components")
	}

	// Check if values feature is enabled when values are passed
	if len(opts.Values) > 0 && !feature.IsEnabled(feature.Values) {
		return fmt.Errorf("package-level values passed in but \"%s\" feature is not enabled."+
			" Run again with --features=\"%s=true\"", feature.Values, feature.Values)
	}

	vals := opts.Values
	if vals == nil {
		vals = value.Values{}
	}

	// Check that cluster is configured if required.
	requiresCluster := false
	componentIdx := map[string]v1alpha1.ZarfComponent{}
	for _, component := range pkg.Components {
		componentIdx[component.Name] = component
		if component.RequiresCluster() {
			if opts.Cluster == nil {
				return fmt.Errorf("component %s requires cluster access but none was configured", component.Name)
			}
			requiresCluster = true
		}
	}

	// Get or build the secret for the deployed package
	depPkg := &state.DeployedPackage{}
	if requiresCluster {
		var err error
		depPkg, err = opts.Cluster.GetDeployedPackage(ctx, pkg.Metadata.Name, state.WithPackageNamespaceOverride(opts.NamespaceOverride))
		if err != nil {
			return fmt.Errorf("unable to load the secret for the package we are attempting to remove: %s", err.Error())
		}
	} else {
		// If we do not need the cluster, create a deployed components object based on the info we have
		depPkg.Name = pkg.Metadata.Name
		depPkg.Data = pkg
		for _, component := range pkg.Components {
			depPkg.DeployedComponents = append(depPkg.DeployedComponents, state.DeployedComponent{Name: component.Name})
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
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
			err := actions.Run(ctx, cwd, comp.Actions.OnRemove.Defaults, comp.Actions.OnRemove.Before, nil, vals)
			if err != nil {
				return fmt.Errorf("unable to run the before action: %w", err)
			}

			reverseInstalledCharts := slices.Clone(depComp.InstalledCharts)
			slices.Reverse(reverseInstalledCharts)
			if opts.Cluster != nil {
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
					client.Timeout = opts.Timeout
					_, err = client.Run(chart.ChartName)
					if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
						return fmt.Errorf("unable to uninstall the helm chart %s in the namespace %s: %w", chart.ChartName, chart.Namespace, err)
					}
					if errors.Is(err, driver.ErrReleaseNotFound) {
						l.Warn("helm release was not found. was it already removed?", "name", chart.ChartName, "namespace", chart.Namespace)
					}

					// remove the helm chart from the installed charts slice.
					depComp.InstalledCharts = helpers.RemoveMatches(depComp.InstalledCharts, func(t state.InstalledChart) bool {
						return t.ChartName == chart.ChartName
					})

					err = opts.Cluster.UpdateDeployedPackage(ctx, *depPkg)
					if err != nil {
						// We warn and ignore errors because we may have removed the cluster that this package was inside of
						l.Warn("unable to update secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
					}
				}
			}

			err = actions.Run(ctx, cwd, comp.Actions.OnRemove.Defaults, comp.Actions.OnRemove.After, nil, vals)
			if err != nil {
				return fmt.Errorf("unable to run the after action: %w", err)
			}
			err = actions.Run(ctx, cwd, comp.Actions.OnRemove.Defaults, comp.Actions.OnRemove.OnSuccess, nil, vals)
			if err != nil {
				return fmt.Errorf("unable to run the success action: %w", err)
			}

			// remove the component from deploy components slice.
			if opts.Cluster != nil {
				depPkg.DeployedComponents = helpers.RemoveMatches(depPkg.DeployedComponents, func(t state.DeployedComponent) bool {
					return t.Name == depComp.Name
				})
				err = opts.Cluster.UpdateDeployedPackage(ctx, *depPkg)
				if err != nil {
					// We warn and ignore errors because we may have removed the cluster that this package was inside of
					l.Warn("unable to update secret package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
				}
			}
			return nil
		}()
		if err != nil {
			removeErr := actions.Run(ctx, cwd, comp.Actions.OnRemove.Defaults, comp.Actions.OnRemove.OnFailure, nil, vals)
			if removeErr != nil {
				return errors.Join(fmt.Errorf("unable to run the failure action: %w", err), removeErr)
			}
			return err
		}
	}

	// All the installed components were deleted, therefore this package is no longer actually deployed
	if opts.Cluster != nil && len(depPkg.DeployedComponents) == 0 {
		err := opts.Cluster.DeleteDeployedPackage(ctx, *depPkg)
		if err != nil {
			l.Warn("unable to delete secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
		}
	}

	l.Info("package successfully removed", "name", pkg.Metadata.Name)
	return nil
}
