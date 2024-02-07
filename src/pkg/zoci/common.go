// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	// ZarfConfigMediaType is the media type for the manifest config
	ZarfConfigMediaType = "application/vnd.zarf.config.v1+json"
	// ZarfLayerMediaTypeBlob is the media type for all Zarf layers due to the range of possible content
	ZarfLayerMediaTypeBlob = "application/vnd.zarf.layer.v1.blob"
	// SkeletonArch is the architecture used for skeleton packages
	SkeletonArch = "skeleton"
)

// Remote is a wrapper around the Oras remote repository with zarf specific functions
type Remote struct {
	*oci.OrasRemote
}

// NewRemote returns an oras remote repository client and context for the given url
// with zarf opination embedded
func NewRemote(url string, platform ocispec.Platform, mods ...oci.Modifier) (*Remote, error) {
	modifiers := append([]oci.Modifier{
		oci.WithPlainHTTP(config.CommonOptions.Insecure),
		oci.WithInsecureSkipVerify(config.CommonOptions.Insecure),
		oci.WithLogger(&message.Logger{}),
		oci.WithUserAgent("zarf/" + config.CLIVersion),
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
