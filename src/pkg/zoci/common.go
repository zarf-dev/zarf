// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"path/filepath"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	orasContent "oras.land/oras-go/v2/content/oci"
)

const (
	// ZarfConfigMediaType is the media type for the manifest config
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// SkeletonArch is the architecture used for skeleton packages
	SkeletonArch = "skeleton"
	// DefaultConcurrency is the default concurrency used for operations
	DefaultConcurrency = 3
	// ImageCacheDirectory is the directory within the Zarf cache containing an OCI store
	ImageCacheDirectory = "images"
)

// Remote is a wrapper around the Oras remote repository with zarf specific functions
type Remote struct {
	*oci.OrasRemote
}

// NewRemote returns an oras remote repository client and context for the given url
// with zarf opination embedded
func NewRemote(ctx context.Context, url string, platform ocispec.Platform, mods ...oci.Modifier) (*Remote, error) {
	l := logger.From(ctx)
	absCachePath, err := config.GetAbsCachePath()
	if err != nil {
		return nil, err
	}
	orasStore, err := orasContent.NewWithContext(ctx, filepath.Join(absCachePath, ImageCacheDirectory))
	if err != nil {
		return nil, err
	}
	modifiers := append([]oci.Modifier{
		oci.WithPlainHTTP(config.CommonOptions.PlainHTTP),
		oci.WithInsecureSkipVerify(config.CommonOptions.InsecureSkipTLSVerify),
		oci.WithLogger(l),
		oci.WithUserAgent("zarf/" + config.CLIVersion),
		oci.WithCache(orasStore),
	}, mods...)
	remote, err := oci.NewOrasRemote(url, platform, modifiers...)
	if err != nil {
		return nil, err
	}
	return &Remote{remote}, nil
}

// PlatformForSkeleton sets the target architecture for the remote to skeleton
func PlatformForSkeleton() ocispec.Platform {
	return ocispec.Platform{
		OS:           oci.MultiOS,
		Architecture: SkeletonArch,
	}
}
