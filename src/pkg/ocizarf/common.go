// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package ocizarf contains functions for interacting with Zarf packages stored in OCI registries.
package ocizarf

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	// ZarfConfigMediaType is the media type for the manifest config
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// SkeletonArch is the architecture used for skeleton packages
	SkeletonArch = "skeleton"
)

// ZarfOrasRemote is a wrapper around the Oras remote repository with zarf specific functions
type ZarfOrasRemote struct {
	*oci.OrasRemote
}

//TODO: take away ctx from ORAS remote
//TODO: Look in ocizarf to see if there's any functions that should be moved outside of oci completely

// NewZarfOrasRemote returns an oras remote repository client and context for the given url
// with zarf opination embedded
func NewZarfOrasRemote(url string, platform ocispec.Platform, mod ...oci.Modifier) (*ZarfOrasRemote, error) {
	modifiers := append(mod, oci.WithMediaType(ZarfConfigMediaType))
	remote, err := oci.NewOrasRemote(url, &message.Logger{}, platform, modifiers...)
	if err != nil {
		return nil, err
	}
	return &ZarfOrasRemote{remote}, nil
}

// PlatformForSkeleton returns a skeleton oci
func PlatformForSkeleton() ocispec.Platform {
	return ocispec.Platform{
		OS:           oci.MultiOS,
		Architecture: SkeletonArch,
	}
}
