// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"path/filepath"
	"time"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	ociDirectory "oras.land/oras-go/v2/content/oci"
)

// LayersSelector is a type for selecting subsets of layers in a Zarf package
type LayersSelector string

const (
	// ZarfConfigMediaType is the media type for the manifest config
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// DefaultConcurrency is the default concurrency used for operations
	DefaultConcurrency = 6
	//DefaultRetries is the default number of retries for operations
	DefaultRetries = 1
	// ImageCacheDirectory is the directory within the Zarf cache containing an OCI store
	ImageCacheDirectory = "images"
	// AllLayers is the default selector for all layers
	AllLayers LayersSelector = ""
	//SbomLayers is the selector for SBOM layers including metadata
	SbomLayers LayersSelector = "sbom"
	// MetadataLayers is the selector for metadata layers (zarf.yaml, signature, checksums)
	MetadataLayers LayersSelector = "metadata"
	// ImageLayers is the selector for image layers including metadata
	ImageLayers LayersSelector = "images"
	// ComponentLayers is the selector for component layers including metadata
	ComponentLayers LayersSelector = "components"
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
}

// Remote is a wrapper around the Oras remote repository with zarf specific functions
type Remote struct {
	*oci.OrasRemote
}

// NewRemote returns an oras remote repository client and context for the given url
// with zarf opination embedded
func NewRemote(ctx context.Context, url string, platform ocispec.Platform, mods ...oci.Modifier) (*Remote, error) {
	l := logger.From(ctx)

	modifiers := append([]oci.Modifier{
		oci.WithPlainHTTP(config.CommonOptions.PlainHTTP),
		oci.WithInsecureSkipVerify(config.CommonOptions.InsecureSkipTLSVerify),
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
