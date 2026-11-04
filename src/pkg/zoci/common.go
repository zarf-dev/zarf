// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
	"github.com/zarf-dev/zarf/src/types"
	ociDirectory "oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// LayerSelection controls optional OCI package layers to retrieve.
// Metadata layers are always retrieved; a zero-value selection retrieves metadata only.
type LayerSelection struct {
	// RequestedComponents scopes component and image layers.
	// A nil slice selects every package component; a non-nil empty slice selects none.
	RequestedComponents []api.Component
	// IncludeComponents retrieves component archive layers in the requested scope.
	IncludeComponents bool
	// IncludeImages retrieves image layers referenced by components in the requested scope.
	IncludeImages bool
	// SBOMs selects SBOM resources; nil skips SBOM layers.
	SBOMs *SBOMSelection
	// Attachments selects attachment resources; nil skips attachment layers.
	Attachments *AttachmentSelection
}

// SBOMSelection specifies which checksum-declared SBOM resources to retrieve.
// Keys are exact component names or image references. A key selects every matching resource.
// An empty key set selects every declared SBOM.
type SBOMSelection struct {
	Keys []string
}

// AttachmentSelection specifies which declared attachment resources to retrieve.
// An empty key set selects every declared attachment.
type AttachmentSelection struct {
	Keys []string
}

// AllLayerSelection returns a selection for every package resource.
func AllLayerSelection() LayerSelection {
	return LayerSelection{
		IncludeComponents: true,
		IncludeImages:     true,
		SBOMs:             &SBOMSelection{},
		Attachments:       &AttachmentSelection{},
	}
}

const (
	// DefaultConcurrency is the default concurrency used for operations.
	DefaultConcurrency = 6
	// DefaultRetries is the default number of retries for operations.
	DefaultRetries = 1
	// ImageCacheDirectory is the directory within the Zarf cache containing OCI blobs.
	ImageCacheDirectory = "images"
)

const (
	defaultDelayTime    = 500 * time.Millisecond
	defaultMaxDelayTime = 8 * time.Second
)

// PublishOptions contains options for the publish operation
type PublishOptions struct {
	// Retries is the number of times to retry a failed operation
	Retries int
	// OCIConcurrency configures the amount of layers to push in parallel
	OCIConcurrency int
	// Tag allows for overriding the destination reference
	Tag string
}

// RemoteClientOptions configures Zarf's OCI remote client.
type RemoteClientOptions struct {
	// CachePath stores OCI layers locally when non-empty.
	CachePath string
	// CredentialFunc supplies registry credentials.
	// If nil, the Docker credential store is used instead.
	CredentialFunc auth.CredentialFunc
	// Transport configures HTTP transport behavior such as proxies and mTLS.
	// It is cloned before use and is never modified.
	Transport *http.Transport
	types.RemoteOptions
}

// Remote is a wrapper around the Oras remote repository with zarf specific functions
type Remote struct {
	*oci.OrasRemote
}

// NewRemote returns an ORAS remote repository client configured for the given URL.
//
// Deprecated: Use NewRemoteWithOptions so Zarf owns transport configuration.
func NewRemote(ctx context.Context, url string, platform ocispec.Platform, mods ...oci.Modifier) (*Remote, error) {
	return newRemote(ctx, url, platform, mods...)
}

// NewRemoteWithOptions returns an ORAS remote repository configured with Zarf's
// cache, credential, and transport options.
func NewRemoteWithOptions(ctx context.Context, url string, platform ocispec.Platform, options RemoteClientOptions) (*Remote, error) {
	modifiers := []oci.Modifier{
		oci.WithInsecureSkipVerify(options.InsecureSkipTLSVerify),
	}
	if options.CredentialFunc != nil {
		modifiers = append(modifiers, oci.WithCredentialFunc(options.CredentialFunc))
	}
	if options.Transport != nil {
		modifiers = append(modifiers, oci.WithTransport(options.Transport))
	}
	if options.CachePath != "" {
		cacheModifier, err := GetOCICacheModifier(options.CachePath)
		if err != nil {
			return nil, err
		}
		modifiers = append(modifiers, cacheModifier)
	}

	remote, err := newRemote(ctx, url, platform, modifiers...)
	if err != nil {
		return nil, err
	}

	// negotiate if required after the remote has been instantiated for any canonical updates (docker.io etc)
	if options.PlainHTTP {
		probeOptions := ocischeme.ProbeOptions{
			InsecureSkipTLSVerify: options.InsecureSkipTLSVerify,
		}
		if options.Transport != nil {
			probeOptions.Transport = transportForProbe(options.Transport, options.InsecureSkipTLSVerify)
		}
		plainHTTP, err := ocischeme.From(ctx).UsePlainHTTP(ctx, remote.Repo().Reference.Registry, probeOptions)
		if err != nil {
			return nil, fmt.Errorf("could not resolve registry transport: %w", err)
		}
		remote.Repo().PlainHTTP = plainHTTP
	}

	return remote, nil
}

// transportForProbe returns a copy of transport with the same TLS verification
// setting that oci.WithInsecureSkipVerify applies to the remote client.
func transportForProbe(transport *http.Transport, insecureSkipTLSVerify bool) *http.Transport {
	if transport == nil {
		return nil
	}
	probeTransport := transport.Clone()
	if probeTransport.TLSClientConfig == nil {
		probeTransport.TLSClientConfig = &tls.Config{}
	} else {
		probeTransport.TLSClientConfig = probeTransport.TLSClientConfig.Clone()
	}
	probeTransport.TLSClientConfig.InsecureSkipVerify = insecureSkipTLSVerify //nolint:gosec // Controlled by --insecure-skip-tls-verify.
	return probeTransport
}

func newRemote(ctx context.Context, url string, platform ocispec.Platform, mods ...oci.Modifier) (*Remote, error) {
	l := logger.From(ctx)
	modifiers := append([]oci.Modifier{
		oci.WithLogger(l),
		oci.WithUserAgent("zarf/" + config.CLIVersion),
	}, mods...)
	remote, err := oci.NewOrasRemote(url, platform, modifiers...)
	if err != nil {
		return nil, err
	}
	return &Remote{remote}, nil
}

// GetOCICacheModifier takes in a Zarf cachePath and uses it to return an oci.WithCache modifier
func GetOCICacheModifier(cachePath string) (oci.Modifier, error) {
	ociCache, err := ociDirectory.NewStorage(filepath.Join(cachePath, ImageCacheDirectory))
	if err != nil {
		return nil, err
	}
	return oci.WithCache(ociCache), nil
}

// PlatformForSkeleton sets the target architecture for the remote to skeleton
func PlatformForSkeleton() ocispec.Platform {
	return ocispec.Platform{
		OS:           oci.MultiOS,
		Architecture: v1alpha1.SkeletonArch,
	}
}
