// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package http provides a http server for the webhook and proxy.
package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/defenseunicorns/zarf/src/internal/agent/hooks"
	"github.com/defenseunicorns/zarf/src/internal/agent/http/admission"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewAdmissionServer creates a http.Server for the mutating webhook admission handler.
func NewAdmissionServer(port string) *http.Server {
	message.Debugf("http.NewAdmissionServer(%s)", port)

	c, err := cluster.NewCluster()
	if err != nil {
		message.Fatalf(err, err.Error())
	}

	ctx := context.Background()

	// Routers
	admissionHandler := admission.NewHandler()
	podsMutation := hooks.NewPodMutationHook(ctx, c)
	fluxGitRepositoryMutation := hooks.NewGitRepositoryMutationHook(ctx, c)
	argocdApplicationMutation := hooks.NewApplicationMutationHook(ctx, c)
	argocdRepositoryMutation := hooks.NewRepositorySecretMutationHook(ctx, c)
	fluxHelmRepositoryMutation := hooks.NewHelmRepositoryMutationHook(ctx, c)
	fluxOCIRepositoryMutation := hooks.NewOCIRepositoryMutationHook(ctx, c)

	// Routers
	mux := http.NewServeMux()
	mux.Handle("/healthz", healthz())
	mux.Handle("/mutate/pod", admissionHandler.Serve(podsMutation))
	mux.Handle("/mutate/flux-gitrepository", admissionHandler.Serve(fluxGitRepositoryMutation))
	mux.Handle("/mutate/flux-helmrepository", admissionHandler.Serve(fluxHelmRepositoryMutation))
	mux.Handle("/mutate/flux-ocirepository", admissionHandler.Serve(fluxOCIRepositoryMutation))
	mux.Handle("/mutate/argocd-application", admissionHandler.Serve(argocdApplicationMutation))
	mux.Handle("/mutate/argocd-repository", admissionHandler.Serve(argocdRepositoryMutation))
	mux.Handle("/metrics", promhttp.Handler())

	return &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // Set ReadHeaderTimeout to avoid Slowloris attacks
	}
}

// NewProxyServer creates and returns an http proxy server.
func NewProxyServer(port string) *http.Server {
	message.Debugf("http.NewHTTPProxy(%s)", port)

	mux := http.NewServeMux()
	mux.Handle("/healthz", healthz())
	mux.Handle("/", ProxyHandler())
	mux.Handle("/metrics", promhttp.Handler())

	return &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // Set ReadHeaderTimeout to avoid Slowloris attacks
	}
}

func healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}
