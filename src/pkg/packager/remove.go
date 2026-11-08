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

	"helm.sh/helm/v4/pkg/storage/driver"

	"github.com/opencontainers/go-digest"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/internal/packager/requirements"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/feature"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/template"
	"github.com/zarf-dev/zarf/src/pkg/value"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

// RemoveOptions are the options for Remove.
type RemoveOptions struct {
	RemoveConfig
	Cluster *cluster.Cluster
	// Timeout for Helm operations.
	Timeout          time.Duration
	SkipVersionCheck bool
}

// Remove runs removal actions and uninstalls recorded Helm charts for components
// selected by the ordered ComponentSelectors list, with the last matching selector
// taking precedence. An empty list selects all components. When a deployment record
// exists, it preserves unselected components and records progress even for local-only
// selections. Standalone local packages without a deployment record never create one.
func Remove(ctx context.Context, definition api.Package, opts RemoveOptions) error {
	if err := definition.Validate(); err != nil {
		return err
	}
	l := logger.From(ctx)
	// Validate operational requirements before proceeding
	if !opts.SkipVersionCheck {
		if err := requirements.ValidateVersionRequirements(definition.Build.VersionRequirements); err != nil {
			return fmt.Errorf("%w If you cannot upgrade Zarf you may skip this check with --skip-version-check. Unexpected behavior or errors may occur", err)
		}
	}

	if opts.Timeout == 0 {
		opts.Timeout = config.ZarfDefaultTimeout
	}

	normalizedSelectors, err := filters.NormalizeComponentSelectorList(opts.ComponentSelectors)
	if err != nil {
		return err
	}
	opts.ComponentSelectors = normalizedSelectors
	removeConfigDigest, err := opts.RemoveConfig.Digest()
	if err != nil {
		return fmt.Errorf("digest remove config: %w", err)
	}
	definition, err = filters.Apply(definition, filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ByComponentSelectors(opts.ComponentSelectors),
	))
	if err != nil {
		return err
	}
	if len(definition.Components) == 0 {
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
	componentIdx := map[string]api.Component{}
	for _, component := range definition.Components {
		componentIdx[component.Name] = component
		if component.RequiresCluster() {
			if opts.Cluster == nil {
				return fmt.Errorf("component %s requires cluster access but none was configured", component.Name)
			}
			requiresCluster = true
		}
	}

	// A local-only selection may still belong to a package recorded in the cluster.
	var depPkg *state.DeployedPackage
	persistState := false
	if opts.Cluster != nil {
		depPkg, err = opts.Cluster.GetDeployedPackage(ctx, definition.Metadata.Name, state.WithPackageNamespaceOverride(opts.NamespaceOverride))
		switch {
		case err == nil:
			persistState = true
		case kerrors.IsNotFound(err) && !requiresCluster:
			// Standalone local packages have no deployment record.
		default:
			return fmt.Errorf("unable to load the secret for the package we are attempting to remove: %w", err)
		}
	}
	if !persistState {
		depPkg = &state.DeployedPackage{Name: definition.Metadata.Name}
		if err := depPkg.SetPackageDefinition(definition); err != nil {
			return err
		}
		for _, component := range definition.Components {
			depPkg.DeployedComponents = append(depPkg.DeployedComponents, state.DeployedComponent{Name: component.Name})
		}
	}

	removePackageEvent := func(outcome state.EventOutcome) state.PackageEvent {
		return state.PackageEvent{
			Type:         state.EventTypeRemove,
			Outcome:      outcome,
			Timestamp:    time.Now(),
			Version:      definition.Metadata.Version,
			Flavor:       definition.Build.Flavor,
			Digest:       digest.Digest(depPkg.Digest),
			ConfigDigest: removeConfigDigest,
		}
	}
	if persistState {
		depPkg.AppendEvent(removePackageEvent(state.EventOutcomeInProgress))
		if err := opts.Cluster.StoreDeployedPackage(ctx, *depPkg); err != nil {
			l.Warn("unable to update secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	var s *state.State
	if opts.Cluster != nil {
		s, err = opts.Cluster.LoadState(ctx)
		if err != nil {
			l.Debug("unable to load Zarf state for remove actions", "error", err.Error())
		}
	}

	for i := len(depPkg.DeployedComponents) - 1; i >= 0; i-- {
		depComp := &depPkg.DeployedComponents[i]
		// Only remove the component if it was requested or if we are removing the whole package.
		comp, ok := componentIdx[depComp.Name]
		if !ok {
			continue
		}

		depComp.Status = state.ComponentStatusRemoving //nolint:staticcheck // Legacy status remains persisted for compatibility.
		depComp.LastEvent = state.ComponentEvent{Type: state.EventTypeRemove, Outcome: state.EventOutcomeInProgress, Timestamp: time.Now()}
		if persistState {
			if err := opts.Cluster.StoreDeployedPackage(ctx, *depPkg); err != nil {
				l.Warn("unable to update secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
			}
		}

		err := func() error {
			stateAccess := template.StateAccess{State: s, AccessKeys: comp.StateAccess}
			err := actions.Run(ctx, cwd, comp.Actions.OnRemove.Before, actions.RunOptions{
				DefaultConfig: comp.Actions.OnRemove.Defaults,
				Values:        vals,
				StateAccess:   stateAccess,
			})
			if err != nil {
				return fmt.Errorf("unable to run the before action: %w", err)
			}

			if persistState {
				for j := len(depComp.InstalledCharts) - 1; j >= 0; j-- {
					chart := depComp.InstalledCharts[j]
					err := helm.RemoveChart(ctx, chart.Namespace, chart.ChartName, opts.Timeout)
					if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
						return fmt.Errorf("unable to uninstall the helm chart %s in the namespace %s: %w", chart.ChartName, chart.Namespace, err)
					}
					if errors.Is(err, driver.ErrReleaseNotFound) {
						l.Warn("helm release was not found. was it already removed?", "name", chart.ChartName, "namespace", chart.Namespace)
					}

					// remove the helm chart from the installed charts slice.
					depComp.InstalledCharts = slices.Delete(depComp.InstalledCharts, j, j+1)

					err = opts.Cluster.StoreDeployedPackage(ctx, *depPkg)
					if err != nil {
						// We warn and ignore errors because we may have removed the cluster that this package was inside of
						l.Warn("unable to update secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
					}
				}
			}

			err = actions.Run(ctx, cwd, comp.Actions.OnRemove.After, actions.RunOptions{
				DefaultConfig: comp.Actions.OnRemove.Defaults,
				Values:        vals,
				StateAccess:   stateAccess,
			})
			if err != nil {
				return fmt.Errorf("unable to run the after action: %w", err)
			}
			err = actions.Run(ctx, cwd, comp.Actions.OnRemove.OnSuccess, actions.RunOptions{
				DefaultConfig: comp.Actions.OnRemove.Defaults,
				Values:        vals,
				StateAccess:   stateAccess,
			})
			if err != nil {
				return fmt.Errorf("unable to run the success action: %w", err)
			}

			// Remove completed components locally even when no deployment record exists.
			depPkg.DeployedComponents = slices.Delete(depPkg.DeployedComponents, i, i+1)
			if persistState {
				err = opts.Cluster.StoreDeployedPackage(ctx, *depPkg)
				if err != nil {
					// We warn and ignore errors because we may have removed the cluster that this package was inside of
					l.Warn("unable to update secret package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
				}
			}
			return nil
		}()
		if err != nil {
			if persistState {
				depComp.Status = state.ComponentStatusFailed //nolint:staticcheck // Legacy status remains persisted for compatibility.
				depComp.LastEvent = state.ComponentEvent{Type: state.EventTypeRemove, Outcome: state.EventOutcomeFailed, Timestamp: time.Now()}
				depPkg.AppendEvent(removePackageEvent(state.EventOutcomeFailed))
				if updateErr := opts.Cluster.StoreDeployedPackage(ctx, *depPkg); updateErr != nil {
					l.Warn("unable to update secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", updateErr.Error())
				}
			}
			stateAccess := template.StateAccess{State: s, AccessKeys: comp.StateAccess}
			removeErr := actions.Run(ctx, cwd, comp.Actions.OnRemove.OnFailure, actions.RunOptions{
				DefaultConfig: comp.Actions.OnRemove.Defaults,
				Values:        vals,
				StateAccess:   stateAccess,
			})
			if removeErr != nil {
				return errors.Join(fmt.Errorf("unable to run the failure action: %w", err), removeErr)
			}
			return err
		}
	}

	if persistState {
		depPkg.AppendEvent(removePackageEvent(state.EventOutcomeSucceeded))
		if len(depPkg.DeployedComponents) == 0 {
			err := opts.Cluster.DeleteDeployedPackage(ctx, *depPkg)
			if err != nil {
				l.Warn("unable to delete secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
			}
		} else if err := opts.Cluster.StoreDeployedPackage(ctx, *depPkg); err != nil {
			l.Warn("unable to update secret for package, this may be normal if the cluster was removed", "pkgName", depPkg.Name, "error", err.Error())
		}
	}

	l.Info("package successfully removed", "name", definition.Metadata.Name)
	return nil
}
