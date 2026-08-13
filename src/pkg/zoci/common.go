// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/ocischeme"
	"github.com/zarf-dev/zarf/src/types"
	ociDirectory "oras.land/oras-go/v2/content/oci"
)

// LayerType specifies a category of layers in a Zarf OCI package.
type LayerType string

const (
	// DefaultConcurrency is the default concurrency used for operations
	DefaultConcurrency = 6
	//DefaultRetries is the default number of retries for operations
	DefaultRetries = 1
	// ImageCacheDirectory is the directory within the Zarf cache containing an OCI store
	ImageCacheDirectory = "images"
	// MetadataLayers includes zarf.yaml, signature, and checksums.
	MetadataLayers LayerType = "metadata"
	// ComponentLayers includes component tarballs.
	ComponentLayers LayerType = "components"
	// ImageLayers includes container image blobs.
	ImageLayers LayerType = "images"
	// SbomLayers includes the SBOM tarball.
	SbomLayers LayerType = "sbom"
	// DocLayers includes the documentation tarball.
	DocLayers LayerType = "documentation"
)

// GetAllLayerTypes returns the complete set of layer types in a Zarf OCI package.
func GetAllLayerTypes() []LayerType {
	return []LayerType{MetadataLayers, ComponentLayers, ImageLayers, SbomLayers, DocLayers}
}

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
// cache and transport options.
func NewRemoteWithOptions(ctx context.Context, url string, platform ocispec.Platform, options RemoteClientOptions) (*Remote, error) {
	modifiers := []oci.Modifier{
		oci.WithInsecureSkipVerify(options.InsecureSkipTLSVerify),
	}
	if options.CachePath != "" {
		cacheModifier, err := GetOCICacheModifier(ctx, options.CachePath)
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
		plainHTTP, err := ocischeme.From(ctx).UsePlainHTTP(ctx, remote.Repo().Reference.Registry, ocischeme.ProbeOptions{
			InsecureSkipTLSVerify: options.InsecureSkipTLSVerify,
		})
		if err != nil {
			return nil, fmt.Errorf("could not resolve registry transport: %w", err)
		}
		oci.WithPlainHTTP(plainHTTP)(remote.OrasRemote)
	}

	return remote, nil
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
func GetOCICacheModifier(ctx context.Context, cachePath string) (oci.Modifier, error) {
	ociCache, err := ociDirectory.NewWithContext(ctx, filepath.Join(cachePath, ImageCacheDirectory))
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
