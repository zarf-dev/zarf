package packager

import (
	"context"
	"fmt"
	"time"

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

type PruneStateResult struct {
	PruneableCharts map[string][]state.InstalledChart
}

// GetPruneableCharts returns a list of installed charts that can be pruned per component
func GetPruneableCharts(deployedPackage *state.DeployedPackage, opts PruneOptions) (PruneStateResult, error) {
	// Validate that if chart is specified, component must also be specified
	if opts.Chart != "" && opts.Component == "" {
		return PruneStateResult{}, fmt.Errorf("component must be specified when chart filter is provided")
	}

	pruneableCharts := make(map[string][]state.InstalledChart, 0)
	foundComponent := opts.Component == ""
	foundChart := opts.Chart == ""

	for _, component := range deployedPackage.DeployedComponents {
		if opts.Component != "" && component.Name != opts.Component {
			continue
		}
		foundComponent = true
		for _, chart := range component.InstalledCharts {
			if opts.Chart != "" && chart.ChartName != opts.Chart {
				continue
			}
			foundChart = true
			if chart.State == state.ChartStateOrphaned {
				pruneableCharts[component.Name] = append(pruneableCharts[component.Name], chart)
			}
		}
	}
	// Validate filters matched something
	if opts.Component != "" && !foundComponent {
		return PruneStateResult{}, fmt.Errorf("component %q not found in deployed package", opts.Component)
	}
	if opts.Chart != "" && !foundChart {
		return PruneStateResult{}, fmt.Errorf("chart %q not found in deployed package", opts.Chart)
	}
	if opts.Chart != "" && foundChart && len(pruneableCharts) == 0 {
		return PruneStateResult{}, fmt.Errorf("chart %q found in deployed package, but is not in the %q state", opts.Chart, state.ChartStateOrphaned)
	}

	return PruneStateResult{PruneableCharts: pruneableCharts}, nil
}

func PruneCharts(ctx context.Context, charts []state.InstalledChart, opts PruneOptions) error {
	for _, chart := range charts {
		err := helm.RemoveChart(ctx, chart.Namespace, chart.ChartName, opts.Timeout)
		if err != nil {
			return err
		}
	}
	return nil
}
