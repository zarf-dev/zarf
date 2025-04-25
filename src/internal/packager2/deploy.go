package packager2

import (
	"context"
	"fmt"
	"time"

	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

type DeployOpts struct {
	SetVariables map[string]string
}

func Deploy(ctx context.Context, pkgLayout *layout.PackageLayout, opts DeployOpts) error {
	l := logger.From(ctx)
	l.Info("starting deploy")
	start := time.Now()
	variableConfig := template.GetZarfVariableConfig(ctx)
	variableConfig.SetConstants(pkgLayout.Pkg.Constants)
	if err := variableConfig.PopulateVariables(pkgLayout.Pkg.Variables, opts.SetVariables); err != nil {
		return fmt.Errorf("unable to populate variables: %w", err)
	}

	hpaModified := false
	var cluster *cluster.Cluster

	// During deploy we disable
	defer resetRegistryHPA(ctx, cluster, hpaModified)
	l.Debug("variables populated", "time", time.Since(start))
	return nil
}

func resetRegistryHPA(ctx context.Context, cluster *cluster.Cluster, hpaModified bool) {
	l := logger.From(ctx)
	if cluster != nil && hpaModified {
		if err := cluster.EnableRegHPAScaleDown(ctx); err != nil {
			l.Debug("unable to reenable the registry HPA scale down", "error", err.Error())
		}
	}
}
