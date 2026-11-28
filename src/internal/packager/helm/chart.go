// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/variables"

	"github.com/avast/retry-go/v4"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	"helm.sh/helm/v4/pkg/storage/driver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/zarf-dev/zarf/src/internal/healthchecks"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
)

// Use same default as Helm CLI does.
const maxHelmHistory = 10

// shouldForceConflicts returns true if ssa is enabled and force conflicts is true
// Zarf won't error if force conflicts is true in a client side apply because a single package could have both csa and ssa
func shouldForceConflicts(ssa api.ServerSideApplyMode, lastRelease release.Accessor, forceConflicts bool) bool {
	if !forceConflicts || ssa == api.ServerSideApplyDisabled {
		return false
	}
	if ssa == api.ServerSideApplyAuto && lastRelease != nil {
		return lastRelease.ApplyMethod() == "ssa"
	}
	return true
}

// InstallUpgradeOptions provide options for the Helm install/upgrade operation
type InstallUpgradeOptions struct {
	// TakeOwnership is true if the chart should adopt existing resources and namespaces
	TakeOwnership bool
	// ForceConflicts causes Helm to take ownership of conflicting fields during Server-Side Apply
	ForceConflicts bool
	// VariableConfig is used to template the variables in the chart
	VariableConfig *variables.VariableConfig
	// State is used to update the registry / git server secrets
	State   *state.State
	Cluster *cluster.Cluster
	// ConnectedDeploy is true when deploying with --connected or YOLO mode, causing resources to be labeled with zarf.dev/agent: ignore
	ConnectedDeploy bool
	// Timeout for the helm install/upgrade
	Timeout time.Duration
	// Retries is the maximum number of attempts for retryable Helm operations.
	Retries int
	// PkgName is the name of the zarf package being installed
	PkgName string
	// NamespaceOverride is the namespace override to use for the chart
	NamespaceOverride string
	// IsInteractive decides if Zarf can interactively prompt users through the CLI
	IsInteractive bool
}

// InstallOrUpgradeChart performs a helm install of the given chart.
func InstallOrUpgradeChart(ctx context.Context, zarfChart api.Chart, chart *chartv2.Chart, values common.Values, opts InstallUpgradeOptions) (state.ConnectStrings, string, error) {
	l := logger.From(ctx)
	start := time.Now()
	source := zarfChart.SourceURL()
	if source == "" {
		source = "Zarf-generated"
	}
	l.Info("processing Helm chart", "name", zarfChart.Name, "source", source)

	// If no release name is specified, use the chart name.
	if zarfChart.ReleaseName == "" {
		zarfChart.ReleaseName = zarfChart.Name
	}
	if opts.VariableConfig == nil {
		opts.VariableConfig = template.GetZarfVariableConfig(ctx, opts.IsInteractive)
	}

	// Setup K8s connection.
	actionConfig, err := createActionConfig(ctx, zarfChart.Namespace)
	if err != nil {
		return nil, zarfChart.ReleaseName, fmt.Errorf("unable to initialize the K8s client: %w", err)
	}

	postRender, err := newRenderer(ctx, zarfChart, opts.TakeOwnership, opts.Cluster, opts.ConnectedDeploy, opts.State, actionConfig, opts.VariableConfig, opts.PkgName, opts.NamespaceOverride)
	if err != nil {
		return nil, zarfChart.ReleaseName, fmt.Errorf("unable to create helm renderer: %w", err)
	}
	histClient := action.NewHistory(actionConfig)

	helmCtx, helmCtxCancel := context.WithTimeout(ctx, opts.Timeout)
	defer helmCtxCancel()

	var newRelease release.Releaser
	err = retryHelmChartOperation(helmCtx, zarfChart.Name, opts.Retries, func() error {
		releases, histErr := histClient.Run(zarfChart.ReleaseName)

		l.Debug("checking for existing helm deployment")

		if errors.Is(histErr, driver.ErrReleaseNotFound) {
			// No prior release, try to install it.
			l.Info("performing Helm install", "chart", zarfChart.Name)

			newRelease, err = installChart(helmCtx, zarfChart, chart, values, opts, actionConfig, postRender)
		} else if histErr == nil && len(releases) > 0 {
			// Otherwise, there is a prior release so upgrade it.
			l.Info("performing Helm upgrade", "chart", zarfChart.Name)

			lastReleaser := releases[len(releases)-1]

			newRelease, err = upgradeChart(helmCtx, zarfChart, chart, values, opts, actionConfig, postRender, lastReleaser)
		} else {
			return fmt.Errorf("unable to verify the chart installation status: %w", histErr)
		}
		return err
	})
	if err != nil {
		removeMsg := "if you need to remove the failed chart, use `zarf package remove`"
		installErr := fmt.Errorf("unable to install chart %w: %s", err, removeMsg)

		releases, err := histClient.Run(zarfChart.ReleaseName)
		if err != nil {
			return nil, zarfChart.ReleaseName, errors.Join(err, installErr)
		}
		var previouslyDeployedRelease release.Accessor
		// Check for previous releases that successfully deployed
		for _, releaser := range releases {
			rel, err := release.NewAccessor(releaser)
			if err != nil {
				return nil, zarfChart.ReleaseName, errors.Join(err, installErr)
			}
			if releasecommon.Status(rel.Status()) == releasecommon.StatusDeployed {
				previouslyDeployedRelease = rel
			}
		}

		// No prior releases means this was an initial install.
		if previouslyDeployedRelease == nil {
			return nil, zarfChart.ReleaseName, installErr
		}

		// Attempt to rollback on a failed upgrade.
		l.Info("performing Helm rollback", "chart", zarfChart.Name)
		err = rollbackChart(zarfChart, previouslyDeployedRelease, actionConfig, opts.Timeout, opts.ForceConflicts)
		if err != nil {
			return nil, zarfChart.ReleaseName, fmt.Errorf("%w: unable to rollback: %w", installErr, err)
		}
		return nil, zarfChart.ReleaseName, installErr
	}

	newRel, err := release.NewAccessor(newRelease)
	if err != nil {
		return nil, zarfChart.ReleaseName, err
	}

	resourceList, err := actionConfig.KubeClient.Build(bytes.NewBufferString(newRel.Manifest()), true)
	if err != nil {
		return nil, zarfChart.ReleaseName, fmt.Errorf("unable to build the resource list: %w", err)
	}

	runtimeObjs := []runtime.Object{}
	for _, resource := range resourceList {
		runtimeObjs = append(runtimeObjs, resource.Object)
	}

	if !zarfChart.SkipWait {
		// Ensure we don't go past the timeout by using a context initialized with the helm timeout
		l.Info("running health checks", "chart", zarfChart.Name)
		if err := healthchecks.WaitForReadyRuntime(helmCtx, opts.Cluster.Watcher, runtimeObjs); err != nil {
			return nil, zarfChart.ReleaseName, err
		}
	}

	l.Debug("done processing Helm chart", "name", zarfChart.Name, "duration", time.Since(start))

	// return any collected connect strings for zarf connect.
	return postRender.connectStrings, zarfChart.ReleaseName, nil
}

func retryHelmChartOperation(ctx context.Context, chartName string, attempts int, operation func() error) error {
	if attempts <= 0 {
		attempts = 1
	}

	l := logger.From(ctx)
	return retry.Do(func() error {
		err := operation()
		if err == nil {
			return nil
		}
		if !isRetryableHelmChartError(err) {
			return retry.Unrecoverable(err)
		}
		return err
	},
		retry.Attempts(uint(attempts)),
		retry.Delay(config.ZarfDefaultRetryDelay),
		retry.MaxDelay(config.ZarfDefaultRetryMaxDelay),
		retry.DelayType(func(n uint, err error, retryConfig *retry.Config) time.Duration {
			delay := retry.BackOffDelay(n, err, retryConfig)
			l.Warn("retrying Helm chart after admission webhook failure",
				"chart", chartName,
				"attempt", n+1,
				"maxAttempts", attempts,
				"nextDelay", delay,
				"error", err,
			)
			return delay
		}),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
	)
}

// isRetryableHelmChartError allows a retry when a just-installed, fail-closed admission webhook is not yet reachable.
func isRetryableHelmChartError(err error) bool {
	if err == nil {
		return false
	}

	if wrapped, ok := err.(interface{ Unwrap() []error }); ok {
		errs := wrapped.Unwrap()
		if len(errs) == 0 {
			return false
		}
		for _, wrappedErr := range errs {
			if !isRetryableHelmChartError(wrappedErr) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return isRetryableHelmChartError(wrapped.Unwrap())
	}
	return isRetryableAdmissionWebhookLeaf(err)
}

func isRetryableAdmissionWebhookLeaf(err error) bool {
	apiStatus, ok := err.(apierrors.APIStatus)
	if !ok {
		return false
	}
	status := apiStatus.Status()
	return status.Reason == metav1.StatusReasonInternalError && strings.Contains(status.Message, "failed calling webhook")
}

// RemoveChart removes a chart from the cluster.
func RemoveChart(ctx context.Context, namespace string, name string, timeout time.Duration) error {
	// Establish a new actionConfig for the namespace.
	actionConfig, err := createActionConfig(ctx, namespace)
	if err != nil {
		return fmt.Errorf("unable to initialize the K8s client: %w", err)
	}
	// Perform the uninstall.
	response, err := uninstallChart(name, actionConfig, timeout)
	logger.From(ctx).Debug("chart uninstalled", "response", response)
	return err
}

// UpdateReleaseValues updates values for a given chart release
// (note: this only works on single-deep charts, charts with dependencies (like loki-stack) will not work)
func UpdateReleaseValues(ctx context.Context, zarfChart api.Chart, updatedValues map[string]interface{}, opts InstallUpgradeOptions) error {
	l := logger.From(ctx)
	l.Debug("updating values for helm release", "name", zarfChart.ReleaseName)

	actionConfig, err := createActionConfig(ctx, zarfChart.Namespace)
	if err != nil {
		return fmt.Errorf("unable to initialize the K8s client: %w", err)
	}
	if opts.VariableConfig == nil {
		opts.VariableConfig = template.GetZarfVariableConfig(ctx, opts.IsInteractive)
	}

	postRender, err := newRenderer(ctx, zarfChart, opts.TakeOwnership, opts.Cluster, opts.ConnectedDeploy, opts.State, actionConfig, opts.VariableConfig, opts.PkgName, opts.NamespaceOverride)
	if err != nil {
		return fmt.Errorf("unable to create helm renderer: %w", err)
	}

	lastReleaser, err := action.NewGet(actionConfig).Run(zarfChart.ReleaseName)
	if err != nil {
		return fmt.Errorf("unable to find the %s helm release: %w", zarfChart.ReleaseName, err)
	}
	lastRelease, err := release.NewAccessor(lastReleaser)
	if err != nil {
		return fmt.Errorf("unable to access release: %w", err)
	}
	// Setup a new upgrade action
	client := action.NewUpgrade(actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = opts.Timeout

	client.SkipCRDs = true

	// Namespace must be specified.
	client.Namespace = zarfChart.Namespace

	// Post-processing our manifests to apply vars and run zarf helm logic in cluster
	client.PostRenderer = postRender

	// Set reuse values to only override the values we are explicitly given
	client.ReuseValues = true

	// Wait for the update operation to successfully complete
	client.WaitStrategy = kube.LegacyStrategy

	client.ServerSideApply = string(zarfChart.GetServerSideApply())
	client.ForceConflicts = shouldForceConflicts(zarfChart.GetServerSideApply(), lastRelease, opts.ForceConflicts)

	// Perform the loadedChart upgrade.
	_, err = client.RunWithContext(ctx, zarfChart.ReleaseName, lastRelease.Chart(), updatedValues)
	if err != nil {
		return err
	}

	return nil
}

func installChart(ctx context.Context, zarfChart api.Chart, chart *chartv2.Chart, chartValues common.Values,
	opts InstallUpgradeOptions, actionConfig *action.Configuration, postRender *renderer) (release.Releaser, error) {
	// Bind the helm action.
	client := action.NewInstall(actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = opts.Timeout

	// Default helm behavior for Zarf is to wait for the resources to deploy, NoWait overrides that for special cases (such as data-injection).
	if zarfChart.SkipWait {
		client.WaitStrategy = kube.HookOnlyStrategy
	} else {
		client.WaitStrategy = kube.LegacyStrategy
	}

	// We need to include CRDs or operator installations will fail spectacularly.
	client.SkipCRDs = false

	// Must be unique per-namespace and < 53 characters. @todo: restrict helm loadedChart name to this.
	client.ReleaseName = zarfChart.ReleaseName

	client.SkipSchemaValidation = zarfChart.SkipSchemaValidation

	// Namespace must be specified.
	client.Namespace = zarfChart.Namespace

	// Post-processing our manifests to apply vars and run zarf helm logic in cluster
	client.PostRenderer = postRender

	client.ServerSideApply = zarfChart.GetServerSideApply() != api.ServerSideApplyDisabled
	client.ForceConflicts = shouldForceConflicts(zarfChart.GetServerSideApply(), nil, opts.ForceConflicts)

	// Adopt pre-existing resources into the release instead of erroring on ownership conflicts.
	client.TakeOwnership = opts.TakeOwnership

	// Perform the loadedChart installation.
	return client.RunWithContext(ctx, chart, chartValues)
}

func upgradeChart(ctx context.Context, zarfChart api.Chart, chart *chartv2.Chart, chartValues common.Values,
	opts InstallUpgradeOptions, actionConfig *action.Configuration, postRender *renderer, lastRelease release.Releaser) (release.Releaser, error) {
	// Setup a new upgrade action
	client := action.NewUpgrade(actionConfig)

	// Let each chart run for the default timeout.
	client.Timeout = opts.Timeout

	// Default helm behavior for Zarf is to wait for the resources to deploy, NoWait overrides that for special cases (such as data-injection).
	if zarfChart.SkipWait {
		client.WaitStrategy = kube.HookOnlyStrategy
	} else {
		client.WaitStrategy = kube.LegacyStrategy
	}

	client.ServerSideApply = string(zarfChart.GetServerSideApply())
	rel, err := release.NewAccessor(lastRelease)
	if err != nil {
		return nil, err
	}
	client.ForceConflicts = shouldForceConflicts(zarfChart.GetServerSideApply(), rel, opts.ForceConflicts)

	// Adopt pre-existing resources into the release instead of erroring on ownership conflicts.
	client.TakeOwnership = opts.TakeOwnership

	client.SkipCRDs = true

	client.SkipSchemaValidation = zarfChart.SkipSchemaValidation

	// Namespace must be specified.
	client.Namespace = zarfChart.Namespace

	// Post-processing our manifests to apply vars and run zarf helm logic in cluster
	client.PostRenderer = postRender

	client.MaxHistory = maxHelmHistory

	// Perform the loadedChart upgrade.
	return client.RunWithContext(ctx, zarfChart.ReleaseName, chart, chartValues)
}

func rollbackChart(zarfChart api.Chart, rel release.Accessor, actionConfig *action.Configuration, timeout time.Duration, forceConflicts bool) error {
	client := action.NewRollback(actionConfig)
	client.CleanupOnFail = true
	client.ServerSideApply = string(zarfChart.GetServerSideApply())
	client.ForceConflicts = shouldForceConflicts(zarfChart.GetServerSideApply(), rel, forceConflicts)
	client.WaitStrategy = kube.LegacyStrategy
	client.Timeout = timeout
	client.Version = rel.Version()
	client.MaxHistory = maxHelmHistory
	return client.Run(zarfChart.ReleaseName)
}

func uninstallChart(name string, actionConfig *action.Configuration, timeout time.Duration) (*release.UninstallReleaseResponse, error) {
	client := action.NewUninstall(actionConfig)
	client.KeepHistory = false
	client.WaitStrategy = kube.LegacyStrategy
	client.Timeout = timeout
	return client.Run(name)
}

// LoadChartData loads a chart from a tarball and returns the Helm SDK representation of the chart and it's values
func LoadChartData(zarfChart api.Chart, paths layout.ChartPaths, valuesOverrides map[string]any) (*chartv2.Chart, common.Values, error) {
	loadedChart, err := loadChartFromTarball(zarfChart, paths)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load chart tarball: %w", err)
	}

	chartValues, err := parseChartValues(zarfChart, paths, valuesOverrides)
	if err != nil {
		return loadedChart, nil, fmt.Errorf("unable to parse chart values: %w", err)
	}
	return loadedChart, chartValues, nil
}
