// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package agent holds the mutating webhook server.
package agent

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/defenseunicorns/zarf/src/config/lang"
	agentHttp "github.com/defenseunicorns/zarf/src/internal/agent/http"
	"github.com/defenseunicorns/zarf/src/pkg/message"
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
func StartWebhook(ctx context.Context) error {
	srv, err := agentHttp.NewAdmissionServer(ctx, httpPort)
	if err != nil {
		return err
	}
	return startServer(ctx, srv)
}

// StartHTTPProxy launches the zarf agent proxy in the cluster.
func StartHTTPProxy(ctx context.Context) error {
	return startServer(ctx, agentHttp.NewProxyServer(httpPort))
}

func startServer(ctx context.Context, srv *http.Server) error {
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
	message.Infof(lang.AgentInfoPort, httpPort)
	err := g.Wait()
	if err != nil {
		return err
	}
	return nil
}
