// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

// Package oci provides tools for interacting with artifacts stored in OCI registries
package oci

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"

	"github.com/defenseunicorns/pkg/helpers/v2"
)

const (
	// MultiOS is the OS used for multi-platform packages
	MultiOS = "multi"
)

// OrasRemote is a wrapper around the Oras remote repository that includes a progress bar for interactive feedback.
type OrasRemote struct {
	repo           *remote.Repository
	cache          *oci.Store
	root           *Manifest
	progTransport  *helpers.Transport
	targetPlatform *ocispec.Platform
	log            *slog.Logger
}

// Modifier is a function that modifies an OrasRemote
type Modifier func(*OrasRemote)

// WithPlainHTTP sets the plain HTTP flag for the remote
func WithPlainHTTP(plainHTTP bool) Modifier {
	return func(o *OrasRemote) {
		o.repo.PlainHTTP = plainHTTP
	}
}

// WithInsecureSkipVerify sets the insecure TLS flag for the remote
func WithInsecureSkipVerify(insecure bool) Modifier {
	return func(o *OrasRemote) {
		transport, ok := o.progTransport.Base.(*http.Transport)
		if ok {
			transport.TLSClientConfig.InsecureSkipVerify = insecure
			return
		}
		if o.log != nil {
			o.log.Warn("unable to set WithInsecureSkipVerify, base transport is not an http.Transport")
		}
	}
}

// PlatformForArch sets the target architecture for the remote
func PlatformForArch(arch string) ocispec.Platform {
	return ocispec.Platform{
		OS:           MultiOS,
		Architecture: arch,
	}
}

// WithUserAgent sets the user agent for the remote
func WithUserAgent(userAgent string) Modifier {
	return func(o *OrasRemote) {
		client, ok := o.repo.Client.(*auth.Client)
		if ok {
			client.SetUserAgent(userAgent)
			return
		}
		if o.log != nil {
			o.log.Warn("unable to set WithUserAgent, client is not an auth.Client")
		}
	}
}

// WithLogger sets the logger for the remote
func WithLogger(logger *slog.Logger) Modifier {
	return func(o *OrasRemote) {
		o.log = logger
	}
}

// WithCache sets the cache for the remote
func WithCache(cache *oci.Store) Modifier {
	return func(o *OrasRemote) {
		o.cache = cache
	}
}

// NewOrasRemote returns an oras remote repository client and context for the given url.
//
// Registry auth is handled by the Docker CLI's credential store and checked before returning the client
func NewOrasRemote(url string, platform ocispec.Platform, mods ...Modifier) (*OrasRemote, error) {
	ref, err := registry.ParseReference(strings.TrimPrefix(url, helpers.OCIURLPrefix))
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCI reference %q: %w", url, err)
	}
	httpTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("http.DefaultTransport is not an *http.Transport, something mutated global net/http variables")
	}
	transport := httpTransport.Clone()
	client := &auth.Client{
		Client: retry.DefaultClient,
		Header: http.Header{
			"User-Agent": {"oras-go"},
		},
		Cache: auth.DefaultCache,
	}
	client.Client.Transport = transport
	o := &OrasRemote{
		repo:           &remote.Repository{Client: client},
		progTransport:  helpers.NewTransport(transport, nil),
		targetPlatform: &platform,
		log:            slog.Default(),
	}

	for _, mod := range mods {
		mod(o)
	}

	if err := o.setRepository(ref); err != nil {
		return nil, err
	}

	return o, nil
}

// SetProgressWriter sets the progress writer for the remote
func (o *OrasRemote) SetProgressWriter(bar helpers.ProgressWriter) {
	o.progTransport.ProgressBar = bar
	client, ok := o.repo.Client.(*auth.Client)
	if ok {
		client.Client.Transport = o.progTransport
		return
	}
	if o.log != nil {
		o.log.Warn("unable to set progress writer, client is not an auth.Client")
	}
}

// ClearProgressWriter clears the progress writer for the remote
func (o *OrasRemote) ClearProgressWriter() {
	o.progTransport.ProgressBar = nil
	client, ok := o.repo.Client.(*auth.Client)
	if ok {
		client.Client.Transport = o.progTransport
		return
	}
	if o.log != nil {
		o.log.Warn("unable to clear progress writer, client is not an auth.Client")
	}
}

// Repo gives you access to the underlying remote repository
func (o *OrasRemote) Repo() *remote.Repository {
	return o.repo
}

// Log gives you access to the OrasRemote logger
func (o *OrasRemote) Log() *slog.Logger {
	return o.log
}

// setRepository sets the repository for the remote as well as the auth client.
func (o *OrasRemote) setRepository(ref registry.Reference) error {
	o.root = nil

	// patch docker.io to registry-1.docker.io
	// this allows end users to use docker.io as an alias for registry-1.docker.io
	if ref.Registry == "docker.io" {
		ref.Registry = "registry-1.docker.io"
	}
	if ref.Registry == "🦄" || ref.Registry == "defenseunicorns" {
		ref.Registry = "ghcr.io"
		ref.Repository = "defenseunicorns/packages/" + ref.Repository
	}
	storeOpts := credentials.StoreOptions{}
	credStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}
	client := &auth.Client{
		Client:     retry.DefaultClient,
		Cache:      auth.NewCache(),
		Credential: credentials.Credential(credStore),
	}
	o.log.Debug("gathering credentials from default Docker config file", "credentials_configured", credStore.IsAuthConfigured())

	o.repo.Reference = ref
	o.repo.Client = client

	return nil
}
