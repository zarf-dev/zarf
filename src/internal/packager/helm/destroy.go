// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"regexp"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/state"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"helm.sh/helm/v3/pkg/action"
)

// Destroy removes ZarfInitPackage charts from the cluster and optionally all Zarf-installed charts.
func Destroy(ctx context.Context, purgeAllZarfInstallations bool) {
	start := time.Now()
	l := logger.From(ctx)
	l.Info("removing Zarf-installed charts")

	// Initially load the actionConfig without a namespace
	actionConfig, err := createActionConfig(ctx, "")
	if err != nil {
		// Don't fatal since this is a removal action
		l.Error("unable to initialize the K8s client", "error", err.Error())
		return
	}

	// Match a name that begins with "zarf-"
	// Explanation: https://regex101.com/r/3yzKZy/1
	zarfPrefix := regexp.MustCompile(`(?m)^zarf-`)

	// Get a list of all releases in all namespaces
	list := action.NewList(actionConfig)
	list.All = true
	list.AllNamespaces = true
	// Uninstall in reverse order
	list.ByDate = true
	list.SortReverse = true
	releases, err := list.Run()
	if err != nil {
		// Don't fatal since this is a removal action
		l.Error("unable to get the list of installed charts", "error", err.Error())
	}

	// Iterate over all releases
	for _, release := range releases {
		if !purgeAllZarfInstallations && release.Namespace != state.ZarfNamespaceName {
			// Don't process releases outside the zarf namespace unless purge all is true
			continue
		}
		// Filter on zarf releases
		if zarfPrefix.MatchString(release.Name) {
			l.Info("uninstalling helm chart", "namespace", release.Namespace, "name", release.Name)
			if err = RemoveChart(ctx, release.Namespace, release.Name, config.ZarfDefaultTimeout); err != nil {
				// Don't fatal since this is a removal action
				l.Error("unable to uninstall the chart", "error", err.Error())
			}
		}
	}
	l.Debug("done uninstalling charts", "duration", time.Since(start))
}
