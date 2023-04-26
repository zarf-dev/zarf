// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-chi/chi/v5"
)

// ListDeployedPackages writes a list of packages that have been deployed to the connected cluster.
func ListDeployedPackages(w http.ResponseWriter, _ *http.Request) {
	c, err := cluster.NewCluster()
	if err != nil {
		message.ErrorWebf(err, w, "Could not connect to cluster")
		return
	}

	deployedPackages, err := c.GetDeployedZarfPackages()
	if err != nil {
		message.ErrorWebf(err, w, "Unable to get a list of the deployed Zarf packages")
		return
	}

	common.WriteJSONResponse(w, deployedPackages, http.StatusOK)
}

// ListPackageConnections lists the zarf connections for a package.
func ListPackageConnections(w http.ResponseWriter, r *http.Request) {
	data := types.APIPackageConnections{}

	pkgName := chi.URLParam(r, "name")

	c, err := cluster.NewCluster()

	if err != nil {
		message.ErrorWebf(err, w, "Could not connect to cluster")
		return
	}

	// Get the package from the cluster.
	pkg, err := c.GetDeployedPackage(pkgName)

	if err != nil {
		message.ErrorWebf(err, w, "Unable to get package %s", pkgName)
		return
	}

	// Get a list of namespaces from the package component charts.
	namespaces := make(map[string]string)
	for _, component := range pkg.DeployedComponents {
		for _, chart := range component.InstalledCharts {
			namespaces[chart.Namespace] = chart.Namespace
		}
	}

	// Get a list of zarf connections from the namespaces.
	connections := make(types.ConnectStrings)
	for namespace := range namespaces {
		// Get a list of services in the namespace with the zarf connect label.
		serviceList, err := c.Kube.GetServicesByLabelExists(namespace, config.ZarfConnectLabelName)

		if err != nil {
			message.ErrorWebf(err, w, "Unable to get a list of the zarf connections for package %s", pkgName)
			return
		}

		for _, svc := range serviceList.Items {
			name := svc.Labels[config.ZarfConnectLabelName]

			// Add the connectString.
			connections[name] = types.ConnectString{
				Description: svc.Annotations[config.ZarfConnectAnnotationDescription],
				URL:         svc.Annotations[config.ZarfConnectAnnotationURL],
			}
		}

	}
	data.ConnectStrings = connections

	common.WriteJSONResponse(w, data, http.StatusOK)
}
