// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// LoadOCIImage returns a v1.Image with the image tag specified from a location provided, or an error if the image cannot be found.
func LoadOCIImage(imgPath, imgTag string) (v1.Image, error) {
	// Use the manifest within the index.json to load the specific image we want
	layoutPath := layout.Path(imgPath)
	imgIdx, err := layoutPath.ImageIndex()
	if err != nil {
		return nil, err
	}
	idxManifest, err := imgIdx.IndexManifest()
	if err != nil {
		return nil, err
	}

	// Search through all the manifests within this package until we find the annotation that matches our tag
	for _, manifest := range idxManifest.Manifests {
		if manifest.Annotations[ocispec.AnnotationBaseImageName] == imgTag {
			// This is the image we are looking for, load it and then return
			return layoutPath.Image(manifest.Digest)
		}
	}

	return nil, fmt.Errorf("unable to find image (%s) at the path (%s)", imgTag, imgPath)
}
