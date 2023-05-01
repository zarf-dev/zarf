// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package tunnels contains the handlers for the tunnels API.
package tunnels

import (
	"net/http"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/go-chi/chi/v5"
)

var tunnels map[string]*cluster.Tunnel

// ListTunnels lists all tunnel names
func ListTunnels(w http.ResponseWriter, _ *http.Request) {
	// make sure tunnels is initialized
	makeTunnels()

	// get the tunnel names
	tunnelNames := make([]string, 0, len(tunnels))
	for name := range tunnels {
		tunnelNames = append(tunnelNames, name)
	}

	common.WriteJSONResponse(w, tunnelNames, http.StatusOK)
}

// ConnectTunnel establishes a tunnel for the requested resource
func ConnectTunnel(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	// make sure tunnels is initialized
	makeTunnels()

	// if the tunnel already exists, just launch the URL
	if tunnels[name] != nil {
		launchTunnelURL(tunnels[name], w, name)
		common.WriteJSONResponse(w, true, http.StatusCreated)
		return
	}

	tunnel, err := cluster.NewZarfTunnel()

	if err != nil {
		message.ErrorWebf(err, w, "Failed to create tunnel for %s", name)
		return
	}

	err = tunnel.Connect(name, false)
	if err != nil {
		message.ErrorWebf(err, w, "Failed to connect to %s", name)
		return
	}

	tunnels[name] = tunnel
	launchTunnelURL(tunnel, w, name)

	common.WriteJSONResponse(w, true, http.StatusOK)
}

// DisconnectTunnel closes the tunnel for the requested resource
func DisconnectTunnel(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	closeTunnel(name)

	common.WriteJSONResponse(w, true, http.StatusOK)
}

// makeTunnels initializes the tunnels map if it is nil
func makeTunnels() {
	if tunnels == nil {
		tunnels = make(map[string]*cluster.Tunnel)
	}
}

// launchTunnelURL launches the tunnel URL in the default browser
func launchTunnelURL(tunnel *cluster.Tunnel, w http.ResponseWriter, name string) {
	if err := exec.LaunchURL(tunnel.FullUrl()); err != nil {
		message.ErrorWebf(err, w, "Failed to launch browser for %s", name)

	}
}

// closeTunnel closes the tunnel for the requested resource and removes it from the tunnels map
func closeTunnel(name string) {
	if tunnels != nil {
		if tunnels[name] != nil {
			tunnels[name].Close()
			delete(tunnels, name)
		}
	}
}
