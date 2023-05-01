// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package cluster

import (
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/defenseunicorns/zarf/src/internal/api/common"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/go-chi/chi"
)

var tunnels map[string]*cluster.Tunnel

// ConnectTunnel establishes a tunnel for the requested resource
func ConnectTunnel(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	if tunnels != nil {
		if tunnels[name] != nil {
			launchTunnelUrl(tunnels[name], w, name)
			common.WriteJSONResponse(w, true, http.StatusCreated)
			return
		}
	} else {
		tunnels = make(map[string]*cluster.Tunnel)
		// Keep this open until an interrupt signal is received.
		c := make(chan os.Signal)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			// Close all tunnels before exiting.
			for _, tunnel := range tunnels {
				tunnel.Close()
			}
			os.Exit(0)
		}()

		for {
			runtime.Gosched()
		}
	}

	tunnel, err := cluster.NewZarfTunnel()

	if err != nil {
		message.ErrorWebf(err, w, "Failed to create tunnel for %s", name)
	}

	err = tunnel.Connect(name, false)
	if err != nil {
		message.ErrorWebf(err, w, "Failed to connect to %s", name)
	}
	tunnels[name] = tunnel
	launchTunnelUrl(tunnel, w, name)

	common.WriteJSONResponse(w, true, http.StatusCreated)
}

// DisconnectTunnel closes the tunnel for the requested resource
func DisconnectTunnel(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	closeTunnel(name)

	common.WriteJSONResponse(w, true, http.StatusOK)
}

// launchTunnelUrl launches the tunnel URL in the default browser
func launchTunnelUrl(tunnel *cluster.Tunnel, w http.ResponseWriter, name string) {
	if err := exec.LaunchURL(tunnel.HTTPEndpoint()); err != nil {
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
