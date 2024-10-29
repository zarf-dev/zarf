// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"context"
	"regexp"

	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"helm.sh/helm/v3/pkg/action"
)

// Destroy removes ZarfInitPackage charts from the cluster and optionally all Zarf-installed charts.
func Destroy(ctx context.Context, purgeAllZarfInstallations bool) {
	spinner := message.NewProgressSpinner("Removing Zarf-installed charts")
	defer spinner.Stop()

	h := Helm{}

	// Initially load the actionConfig without a namespace
	err := h.createActionConfig(ctx, "", spinner)
	if err != nil {
		// Don't fatal since this is a removal action
		spinner.Errorf(err, "Unable to initialize the K8s client")
		return
	}

	// Match a name that begins with "zarf-"
	// Explanation: https://regex101.com/r/3yzKZy/1
	zarfPrefix := regexp.MustCompile(`(?m)^zarf-`)

	// Get a list of all releases in all namespaces
	list := action.NewList(h.actionConfig)
	list.All = true
	list.AllNamespaces = true
	// Uninstall in reverse order
	list.ByDate = true
	list.SortReverse = true
	releases, err := list.Run()
	if err != nil {
		// Don't fatal since this is a removal action
		spinner.Errorf(err, "Unable to get the list of installed charts")
	}

	// Iterate over all releases
	for _, release := range releases {
		if !purgeAllZarfInstallations && release.Namespace != cluster.ZarfNamespaceName {
			// Don't process releases outside the zarf namespace unless purge all is true
			continue
		}
		// Filter on zarf releases
		if zarfPrefix.MatchString(release.Name) {
			spinner.Updatef("Uninstalling helm chart %s/%s", release.Namespace, release.Name)
			if err = h.RemoveChart(ctx, release.Namespace, release.Name, spinner); err != nil {
				// Don't fatal since this is a removal action
				spinner.Errorf(err, "Unable to uninstall the chart")
			}
		}
	}

	spinner.Success()
}
