// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ImgConfig is the main struct for managing container images.
type ImgConfig struct {
	ImagesPath string

	ImgList []string

	RegInfo types.RegistryInfo

	NoChecksum bool

	Insecure bool
}

// GetLegacyImgTarballPath returns the ImagesPath as if it were a path to a tarball instead of a directory.
func (i *ImgConfig) GetLegacyImgTarballPath() string {
	return fmt.Sprintf("%s.tar", i.ImagesPath)
}

// LoadImageFromPackage returns a v1.Image from the image tag specified, or an error if the image cannot be found.
func (i ImgConfig) LoadImageFromPackage(imgTag string) (v1.Image, error) {
	// If the package still has a images.tar that contains all of the images, use crane to load the specific tag we want
	if _, statErr := os.Stat(i.GetLegacyImgTarballPath()); statErr == nil {
		return crane.LoadTag(i.GetLegacyImgTarballPath(), imgTag, config.GetCraneOptions(i.Insecure)...)
	}

	// Load the image from the OCI formatted images directory
	return LoadImage(i.ImagesPath, imgTag)
}

// LoadImage returns a v1.Image with the image tag specified from a location provided, or an error if the image cannot be found.
func LoadImage(imgPath, imgTag string) (v1.Image, error) {
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
