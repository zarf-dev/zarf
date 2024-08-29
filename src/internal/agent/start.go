// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package agent holds the mutating webhook server.
package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"github.com/zarf-dev/zarf/src/internal/agent/hooks"
	agentHttp "github.com/zarf-dev/zarf/src/internal/agent/http"
	"github.com/zarf-dev/zarf/src/internal/agent/http/admission"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logging"
)

// Heavily influenced by https://github.com/douglasmakey/admissioncontroller and
// https://github.com/slackhq/simple-kubernetes-webhook

// We can hard-code these because we control the entire thing anyway.
const (
	httpPort = "8443"
	tlsCert  = "/etc/certs/tls.crt"
	tlsKey   = "/etc/certs/tls.key"
)

// StartWebhook launches the Zarf agent mutating webhook in the cluster.
func StartWebhook(ctx context.Context, cluster *cluster.Cluster) error {
	// Routers
	admissionHandler := admission.NewHandler()
	podsMutation := hooks.NewPodMutationHook(ctx, cluster)
	fluxGitRepositoryMutation := hooks.NewGitRepositoryMutationHook(ctx, cluster)
	argocdApplicationMutation := hooks.NewApplicationMutationHook(ctx, cluster)
	argocdRepositoryMutation := hooks.NewRepositorySecretMutationHook(ctx, cluster)
	fluxHelmRepositoryMutation := hooks.NewHelmRepositoryMutationHook(ctx, cluster)
	fluxOCIRepositoryMutation := hooks.NewOCIRepositoryMutationHook(ctx, cluster)

	// Routers
	mux := http.NewServeMux()
	mux.Handle("/mutate/pod", admissionHandler.Serve(podsMutation))
	mux.Handle("/mutate/flux-gitrepository", admissionHandler.Serve(fluxGitRepositoryMutation))
	mux.Handle("/mutate/flux-helmrepository", admissionHandler.Serve(fluxHelmRepositoryMutation))
	mux.Handle("/mutate/flux-ocirepository", admissionHandler.Serve(fluxOCIRepositoryMutation))
	mux.Handle("/mutate/argocd-application", admissionHandler.Serve(argocdApplicationMutation))
	mux.Handle("/mutate/argocd-repository", admissionHandler.Serve(argocdRepositoryMutation))

	return startServer(ctx, httpPort, mux)
}

// StartHTTPProxy launches the zarf agent proxy in the cluster.
func StartHTTPProxy(ctx context.Context, cluster *cluster.Cluster) error {
	mux := http.NewServeMux()
	mux.Handle("/", agentHttp.ProxyHandler(ctx, cluster))
	return startServer(ctx, httpPort, mux)
}

func startServer(ctx context.Context, port string, mux *http.ServeMux) error {
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint: errcheck // ignore
		w.Write([]byte("ok"))
	}))
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second, // Set ReadHeaderTimeout to avoid Slowloris attacks
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err := srv.ListenAndServeTLS(tlsCert, tlsKey)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-gCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			return err
		}
		return nil
	})
	logging.FromContextOrDiscard(ctx).Info("server running", "port", httpPort)
	err := g.Wait()
	if err != nil {
		return err
	}
	return nil
}
