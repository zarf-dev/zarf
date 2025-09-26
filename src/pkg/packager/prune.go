package packager

import (
	"context"
	"fmt"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/helm"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

type PruneOptions struct {
	Cluster           *cluster.Cluster
	Component         string
	Chart             string
	Timeout           time.Duration
	NamespaceOverride string
	Pending           bool
}

// GetPruneableCharts returns a list of installed charts that can be pruned
func GetPruneableCharts(ctx context.Context, pkg v1alpha1.ZarfPackage, opts PruneOptions) ([]state.InstalledChart, error) {
	deployedPackage, err := opts.Cluster.GetDeployedPackage(ctx, pkg.Metadata.Name, state.WithPackageNamespaceOverride(opts.NamespaceOverride))
	if err != nil {
		return nil, err
	}

	// Determine if we can prune by component and chart
	if opts.Component != "" && opts.Chart != "" {
		return getPruneableChartsByComponentAndChart(ctx, *deployedPackage, opts.Component, opts.Chart)
	}
	// Otherwise determine all pruneable charts
	pruneableCharts := make([]state.InstalledChart, 0, len(deployedPackage.DeployedComponents))

	for _, component := range deployedPackage.DeployedComponents {
		for _, chart := range component.InstalledCharts {
			if opts.Pending {
				if chart.State == state.ChartStatePending {
					pruneableCharts = append(pruneableCharts, chart)
				}
			}
			if chart.State == state.ChartStateOrphaned {
				pruneableCharts = append(pruneableCharts, chart)
			}
		}
	}
	return pruneableCharts, nil
}

func getPruneableChartsByComponentAndChart(ctx context.Context, pkg state.DeployedPackage, component string, chart string) ([]state.InstalledChart, error) {
	for _, deployedComponent := range pkg.DeployedComponents {
		if deployedComponent.Name == component {
			for _, installedChart := range deployedComponent.InstalledCharts {
				if installedChart.ChartName == chart {
					return []state.InstalledChart{installedChart}, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("no installed chart found for component %s and chart %s", component, chart)
}

func Prune(ctx context.Context, charts []state.InstalledChart, opts PruneOptions) error {
	for _, chart := range charts {
		err := helm.RemoveChart(ctx, chart.Namespace, chart.ChartName, opts.Timeout)
		if err != nil {
			return err
		}
	}
	return nil
}
