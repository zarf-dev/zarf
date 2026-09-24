// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/internal/dns"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
	orasRegistry "oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const registryResponseHeaderTimeout = 10 * time.Second

// registryConnection bundles the reference, auth client, tunnel, and resolved scheme needed to operate on a repo or image.
type registryConnection struct {
	ref            string
	client         *auth.Client
	tunnel         *cluster.Tunnel
	plainHTTP      bool
	plainHTTPKnown bool
}

// registryHost returns the registry host:port for repoRef.
func registryHost(repoRef string) (string, error) {
	ref, err := orasRegistry.ParseReference(repoRef)
	if err != nil {
		return "", fmt.Errorf("parsing repo %q: %w", repoRef, err)
	}
	return ref.Host(), nil
}

// setupRegistryAuth builds an ORAS auth client for repoRef, transparently tunneling to and authenticating with a Zarf-managed registry when repoRef targets one, and otherwise falling back to the default Docker credential store.
func setupRegistryAuth(ctx context.Context, repoRef string, plainHTTP, insecureSkipTLSVerify bool) (registryConnection, error) {
	l := logger.From(ctx)

	client, err := images.NewAuthClientFromDocker(ctx, insecureSkipTLSVerify, registryResponseHeaderTimeout, nil)
	if err != nil {
		return registryConnection{}, err
	}
	conn := registryConnection{ref: repoRef, client: client}

	c, err := cluster.New(ctx)
	if err != nil {
		// Not connected to a Zarf-managed cluster; use the default Docker credentials.
		return conn, nil
	}

	l.Info("retrieving registry information from Zarf state")

	s, err := c.LoadState(ctx)
	if err != nil {
		l.Warn("could not get Zarf state from Kubernetes cluster, continuing without state information", "error", err.Error())
		return conn, nil
	}

	// Check to see if it matches the existing internal address.
	if !strings.HasPrefix(repoRef, s.RegistryInfo.Address) {
		return conn, nil
	}

	endpoint, tunnel, err := c.ConnectToZarfRegistryEndpoint(ctx, s.RegistryInfo)
	if err != nil {
		return registryConnection{}, err
	}
	conn.tunnel = tunnel

	if tunnel != nil {
		l.Info("opening a tunnel to the Zarf registry", "localEndpoint", endpoint, "clusterAddress", s.RegistryInfo.Address)
		givenAddress := fmt.Sprintf("%s/", s.RegistryInfo.Address)
		tunnelAddress := fmt.Sprintf("%s/", endpoint)
		conn.ref = strings.Replace(repoRef, givenAddress, tunnelAddress, 1)
	}

	credentialHost, err := registryHost(conn.ref)
	if err != nil {
		return registryConnection{}, err
	}

	client.Credential = auth.StaticCredential(credentialHost, auth.Credential{
		Username: s.RegistryInfo.PushUsername,
		Password: s.RegistryInfo.PushPassword,
	})

	if s.RegistryInfo.ShouldUseMTLS() {
		t, err := getZarfRegistryMTLSTransport(ctx, c)
		if err != nil {
			return registryConnection{}, err
		}
		client.Client.Transport = t
	}

	resolvedPlainHTTP, err := s.RegistryInfo.ResolvePlainHTTP(ctx, credentialHost, plainHTTP, ocischeme.ProbeOptions{InsecureSkipTLSVerify: insecureSkipTLSVerify})
	if err != nil {
		return registryConnection{}, err
	}
	conn.plainHTTP = resolvedPlainHTTP
	conn.plainHTTPKnown = true

	return conn, nil
}

// resolveConnPlainHTTP decides whether repoHost should be reached over plain HTTP: it trusts
// conn's already-known scheme (resolved from Zarf state in setupRegistryAuth) if there is one,
// otherwise honors an explicitly forced plainHTTP, otherwise probes local/private hosts and
// assumes HTTPS for everything else.
func resolveConnPlainHTTP(ctx context.Context, conn registryConnection, repoHost string, plainHTTP, insecureSkipTLSVerify bool) (bool, error) {
	switch {
	case conn.plainHTTPKnown:
		return conn.plainHTTP, nil
	case plainHTTP:
		return true, nil
	case dns.IsLocalOrPrivate(repoHost):
		resolved, err := ocischeme.From(ctx).UsePlainHTTP(ctx, repoHost, ocischeme.ProbeOptions{InsecureSkipTLSVerify: insecureSkipTLSVerify})
		if err != nil {
			return false, fmt.Errorf("probing scheme for %s: %w", repoHost, err)
		}
		return resolved, nil
	}
	return false, nil
}
