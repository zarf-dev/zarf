// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides functions for building and pushing images.
package images

import (
	"fmt"
	"os"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// ImgConfig is the main struct for managing container images.
type ImgConfig struct {
	ImagesPath string

	ImgList []string

	RegInfo types.RegistryInfo

	NoChecksum bool

	Insecure bool

	Architectures []string
}

// GetLegacyImgTarballPath returns the ImagesPath as if it were a path to a tarball instead of a directory.
func (i *ImgConfig) GetLegacyImgTarballPath() string {
	return fmt.Sprintf("%s.tar", i.ImagesPath)
}

// LoadImageFromPackage returns a v1.Image from the image tag specified, or an error if the image cannot be found.
func (i ImgConfig) LoadImageFromPackage(imgTag string) (v1.Image, error) {
	// If the package still has a images.tar that contains all of the images, use crane to load the specific tag we want
	if _, statErr := os.Stat(i.GetLegacyImgTarballPath()); statErr == nil {
		return crane.LoadTag(i.GetLegacyImgTarballPath(), imgTag, config.GetCraneOptions(i.Insecure, i.Architectures...)...)
	}

	// Load the image from the OCI formatted images directory
	return utils.LoadOCIImage(i.ImagesPath, imgTag)
}
