// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	zarfconfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

const (
	// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// ZarfConfigMediaType is the media type for the manifest config
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// SkeletonSuffix is the reference suffix used for skeleton packages
	SkeletonSuffix = "skeleton"
)

// OrasRemote is a wrapper around the Oras remote repository that includes a progress bar for interactive feedback.
type OrasRemote struct {
	repo           *remote.Repository
	root           *ZarfOCIManifest
	ctx            context.Context
	Transport      *utils.Transport
	CopyOpts       oras.CopyOptions
	targetPlatform *ocispec.Platform
}

// Modifier is a function that modifies an OrasRemote
type Modifier func(*OrasRemote)

// NewOrasRemote returns an oras remote repository client and context for the given url.
//
// Registry auth is handled by the Docker CLI's credential store and checked before returning the client
func NewOrasRemote(url string, mods ...Modifier) (*OrasRemote, error) {
	ref, err := registry.ParseReference(strings.TrimPrefix(url, helpers.OCIURLPrefix))
	if err != nil {
		return nil, fmt.Errorf("failed to parse OCI reference %q: %w", url, err)
	}
	o := &OrasRemote{}

	if err := o.setRepository(ref); err != nil {
		return nil, err
	}

	copyOpts := oras.DefaultCopyOptions
	copyOpts.OnCopySkipped = o.printLayerSkipped
	copyOpts.PostCopy = o.printLayerCopied
	o.CopyOpts = copyOpts

	// right now --insecure is overloaded to mean both plain HTTP and insecure TLS
	// putting this here as the "default" for the remote
	// but can be overridden by a provided modifier
	insecureMod := WithInsecureSkipVerify(zarfconfig.CommonOptions.Insecure)
	insecureMod(o)

	httpMod := WithPlainHTTP(zarfconfig.CommonOptions.Insecure)
	httpMod(o)

	for _, mod := range mods {
		mod(o)
	}

	// if no context is provided, use the default
	if o.ctx == nil {
		o.ctx = context.TODO()
	}

	return o, nil
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
	client, err := o.createAuthClient(ref)
	if err != nil {
		return err
	}

	repo, err := remote.NewRepository(ref.String())
	if err != nil {
		return err
	}
	repo.Client = client
	o.repo = repo

	return nil
}

// createAuthClient returns an auth client for the given reference.
//
// The credentials are pulled using Docker's default credential store.
//
// TODO: instead of using Docker's cred store, should use the new one from ORAS to remove that dep
func (o *OrasRemote) createAuthClient(ref registry.Reference) (*auth.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	o.Transport = utils.NewTransport(transport, nil)

	client := &auth.Client{
		Cache: auth.DefaultCache,
		Client: &http.Client{
			Transport: o.Transport,
		},
	}
	client.SetUserAgent("zarf/" + zarfconfig.CLIVersion)

	message.Debugf("Loading docker config file from default config location: %s for %s", config.Dir(), ref)
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return nil, err
	}
	if !cfg.ContainsAuth() {
		message.Debug("no docker config file found, run 'zarf tools registry login --help'")
		return client, nil
	}

	configs := []*configfile.ConfigFile{cfg}

	var key = ref.Registry
	if key == "registry-1.docker.io" {
		// Docker stores its credentials under the following key, otherwise credentials use the registry URL
		key = "https://index.docker.io/v1/"
	}

	authConf, err := configs[0].GetCredentialsStore(key).Get(key)
	if err != nil {
		return nil, fmt.Errorf("unable to get credentials for %s: %w", key, err)
	}

	cred := auth.Credential{
		Username:     authConf.Username,
		Password:     authConf.Password,
		AccessToken:  authConf.RegistryToken,
		RefreshToken: authConf.IdentityToken,
	}

	client.Credential = auth.StaticCredential(ref.Registry, cred)

	return client, nil
}

// WithContext sets the context for the remote
func WithContext(ctx context.Context) Modifier {
	return func(o *OrasRemote) {
		o.ctx = ctx
	}
}

// WithPlainHTTP sets the plain HTTP flag for the remote
func WithPlainHTTP(plainHTTP bool) Modifier {
	return func(o *OrasRemote) {
		o.repo.PlainHTTP = plainHTTP
	}
}

// WithInsecureSkipVerify sets the insecure TLS flag for the remote
func WithInsecureSkipVerify(insecure bool) Modifier {
	return func(o *OrasRemote) {
		o.Transport.Base.(*http.Transport).TLSClientConfig.InsecureSkipVerify = insecure
	}
}

// WithTargetPlatform sets the target platform for the remote
func WithTargetPlatform(platform *ocispec.Platform) Modifier {
	return func(o *OrasRemote) {
		o.targetPlatform = platform
	}
}

// WithSkeletonArch sets the target architecture for the remote to skeleton
func WithSkeletonArch() Modifier {
	return WithTargetPlatform(&ocispec.Platform{
		Architecture: "skeleton",
	})
}

// WithArch sets the target architecture for the remote
func WithArch(arch string) Modifier {
	return WithTargetPlatform(&ocispec.Platform{
		Architecture: arch,
	})
}

// Repo gives you access to the underlying remote repository
func (o *OrasRemote) Repo() *remote.Repository {
	return o.repo
}
