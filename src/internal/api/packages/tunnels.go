// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packages provides api functions for managing Zarf packages.
package packages

import (
	"errors"
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/go-chi/chi/v5"
)

type PackageTunnel struct {
	tunnel     *cluster.Tunnel
	Connection types.APIDeployedPackageConnection `json:"connection,omitempty"`
}
type PackageTunnels map[string]map[string]PackageTunnel

// tunnels is a map of package names to tunnel objects used for storing connected tunnels
var tunnels = make(PackageTunnels)

// ListConnections returns a map of pkgName to a list of connections
func ListConnections(w http.ResponseWriter, _ *http.Request) {
	allConnections := make(types.APIConnections)
	for name, pkgTunnels := range tunnels {
		for _, pkgTunnel := range pkgTunnels {
			if allConnections[name] == nil {
				allConnections[name] = make(types.APIDeployedPackageConnections, 0)
			}
			allConnections[name] = append(allConnections[name], pkgTunnel.Connection)
		}
	}
	common.WriteJSONResponse(w, allConnections, http.StatusOK)
}

// ListPackageConnections lists all tunnel names
func ListPackageConnections(w http.ResponseWriter, r *http.Request) {
	pkgName := chi.URLParam(r, "pkg")
	if tunnels[pkgName] == nil {
		message.ErrorWebf(errors.New("No tunnels for package %s"), w, pkgName)
		return
	}
	pkgTunnels := make(types.APIDeployedPackageConnections, 0, len(tunnels[pkgName]))
	for _, pkgTunnel := range tunnels[pkgName] {
		pkgTunnels = append(pkgTunnels, pkgTunnel.Connection)
	}

	common.WriteJSONResponse(w, pkgTunnels, http.StatusOK)
}

// ConnectTunnel establishes a tunnel for the requested resource
func ConnectTunnel(w http.ResponseWriter, r *http.Request) {
	pkgName := chi.URLParam(r, "pkg")
	connectionName := chi.URLParam(r, "name")

	if tunnels[pkgName] == nil {
		tunnels[pkgName] = make(map[string]PackageTunnel)
	}

	pkgTunnels := tunnels[pkgName]

	if pkgTunnels[connectionName].tunnel != nil {
		common.WriteJSONResponse(w, tunnels[pkgName][connectionName].Connection, http.StatusOK)
		return
	}

	tunnel, err := cluster.NewZarfTunnel()

	if err != nil {
		message.ErrorWebf(err, w, "Failed to create tunnel for %s", connectionName)
		return
	}

	err = tunnel.Connect(connectionName, false)
	if err != nil {
		message.ErrorWebf(err, w, "Failed to connect to %s", connectionName)
		return
	}

	tunnels[pkgName][connectionName] = PackageTunnel{
		tunnel: tunnel,
		Connection: types.APIDeployedPackageConnection{
			Name: connectionName,
			URL:  tunnel.FullURL(),
		},
	}
	common.WriteJSONResponse(w, tunnels[pkgName][connectionName].Connection, http.StatusCreated)
}

// DisconnectTunnel closes the tunnel for the requested resource
func DisconnectTunnel(w http.ResponseWriter, r *http.Request) {
	pkgName := chi.URLParam(r, "pkg")
	connectionName := chi.URLParam(r, "name")
	pkgTunnel := tunnels[pkgName][connectionName]
	if pkgTunnel.tunnel == nil {
		message.ErrorWebf(errors.New("Tunnel not found"), w, "Failed to disconnect from %s", connectionName)
		return
	}

	pkgTunnel.tunnel.Close()
	delete(tunnels[pkgName], connectionName)

	common.WriteJSONResponse(w, true, http.StatusOK)
}
