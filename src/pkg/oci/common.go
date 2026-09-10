// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci provides Zarf's ORAS registry client and OCI descriptor helpers.
package oci

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	orasOCI "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	// MultiOS is the OS used for multi-platform packages
	MultiOS = "multi"
)

// OrasRemote is a wrapper around the Oras remote repository that includes a progress bar for interactive feedback.
type OrasRemote struct {
	repo           *remote.Repository
	cache          *orasOCI.Store
	root           *Manifest
	targetPlatform *ocispec.Platform
	insecure       *bool
	log            *slog.Logger
}

// Modifier is a function that modifies an OrasRemote
type Modifier func(*OrasRemote)

// WithPlainHTTP sets the plain HTTP flag for the remote// WithPlainHTTP sets the plain HTTP flag for the remote
func WithPlainHTTP(plainHTTP bool) Modifier {
	return func(remote *OrasRemote) { remote.repo.PlainHTTP = plainHTTP }
}

// WithInsecureSkipVerify sets the insecure TLS flag for the remote.
// An explicit value takes precedence over WithTransport regardless of modifier order.
func WithInsecureSkipVerify(insecure bool) Modifier {
	return func(remote *OrasRemote) {
		remote.insecure = &insecure
		client, ok := remote.repo.Client.(*auth.Client)
		if !ok {
			return
		}
		transport, ok := client.Client.Transport.(*http.Transport)
		if !ok {
			return
		}
		transport = transport.Clone()
		applyInsecureSkipVerify(transport, insecure)
		client.Client.Transport = transport
	}
}

// WithTransport sets the HTTP transport for the remote.
func WithTransport(transport *http.Transport) Modifier {
	return func(remote *OrasRemote) {
		if transport == nil {
			return
		}
		client, ok := remote.repo.Client.(*auth.Client)
		if ok {
			transport = transport.Clone()
			if remote.insecure != nil {
				applyInsecureSkipVerify(transport, *remote.insecure)
			}
			client.Client.Transport = transport
		}
	}
}

// WithUserAgent sets the user agent for the remote
func WithUserAgent(userAgent string) Modifier {
	return func(remote *OrasRemote) {
		if client, ok := remote.repo.Client.(*auth.Client); ok {
			client.SetUserAgent(userAgent)
		}
	}
}

// WithLogger sets the logger for the remote
func WithLogger(logger *slog.Logger) Modifier {
	return func(remote *OrasRemote) { remote.log = logger }
}

// WithCache sets the cache for the remote
func WithCache(cache *orasOCI.Store) Modifier {
	return func(remote *OrasRemote) { remote.cache = cache }
}

// PlatformForArch sets the target architecture for the remote
func PlatformForArch(arch string) ocispec.Platform {
	return ocispec.Platform{
		OS:           MultiOS,
		Architecture: arch,
	}
}

// NewOrasRemote returns an oras remote repository client and context for the given url.
//
// Registry auth is handled by the Docker CLI's credential store and checked before returning the client
func NewOrasRemote(url string, platform ocispec.Platform, modifiers ...Modifier) (*OrasRemote, error) {
	ref, err := registry.ParseReference(strings.TrimPrefix(url, helpers.OCIURLPrefix))
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCI reference %q: %w", url, err)
	}
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("http.DefaultTransport is not an *http.Transport")
	}
	client := &auth.Client{
		Client: &http.Client{Transport: defaultTransport.Clone()},
		Header: http.Header{"User-Agent": {"oras-go"}},
		Cache:  auth.NewCache(),
	}
	remote := &OrasRemote{
		repo:           &remote.Repository{Client: client},
		targetPlatform: &platform,
		log:            slog.Default(),
	}
	for _, modifier := range modifiers {
		modifier(remote)
	}
	if err := remote.setRepository(ref); err != nil {
		return nil, err
	}
	return remote, nil
}

// Repo gives you access to the underlying remote repository
func (remote *OrasRemote) Repo() *remote.Repository {
	return remote.repo
}

// Log gives you access to the OrasRemote logger
func (remote *OrasRemote) Log() *slog.Logger {
	return remote.log
}

// setRepository sets the repository for the remote as well as the auth client.
func (remote *OrasRemote) setRepository(ref registry.Reference) error {
	remote.root = nil
	if ref.Registry == "docker.io" {
		ref.Registry = "registry-1.docker.io"
	}
	if ref.Registry == "🦄" || ref.Registry == "defenseunicorns" {
		ref.Registry = "ghcr.io"
		ref.Repository = "defenseunicorns/packages/" + ref.Repository
	}
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}
	client, ok := remote.repo.Client.(*auth.Client)
	if !ok {
		return fmt.Errorf("repository client is not an auth client")
	}
	client.Credential = credentials.Credential(store)
	remote.repo.Reference = ref
	return nil
}

// applyInsecureSkipVerify sets InsecureSkipVerify on the TLSClientConfig
func applyInsecureSkipVerify(transport *http.Transport, insecure bool) {
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.InsecureSkipVerify = insecure //nolint:gosec // Configured by the caller.
}
