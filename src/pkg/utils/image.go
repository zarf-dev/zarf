// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"errors"
	"fmt"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	docker_types "github.com/docker/cli/cli/config/types"
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

// SaveDockerCredential saves the provided docker auth config to the default docker.config file.
func SaveDockerCredential(credentialKey string, authConfig docker_types.AuthConfig) error {
	// Load the default docker.config file
	cfg, err := config.Load(config.Dir())
	if err != nil {
		return err
	}
	if !cfg.ContainsAuth() {
		return errors.New("no docker config file found, run 'zarf tools registry login --help'")
	}

	// Save the credentials to the docker.config file
	configs := []*configfile.ConfigFile{cfg}
	err = configs[0].GetCredentialsStore(credentialKey).Store(authConfig)
	if err != nil {
		return fmt.Errorf("unable to get credentials for %s: %w", credentialKey, err)
	}

	return nil
}
