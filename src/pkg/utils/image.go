// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// IndexJSON represents the index.json file in an OCI layout.
type IndexJSON struct {
	SchemaVersion int `json:"schemaVersion"`
	Manifests     []struct {
		MediaType   string            `json:"mediaType"`
		Size        int               `json:"size"`
		Digest      string            `json:"digest"`
		Annotations map[string]string `json:"annotations"`
	} `json:"manifests"`
}

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

// AddImageNameAnnotation adds an annotation to the index.json file so that the deploying code can figure out what the image tag <-> digest shasum will be.
func AddImageNameAnnotation(ociPath string, tagToDigest map[string]string) error {
	indexPath := filepath.Join(ociPath, "index.json")

	// Read the file contents and turn it into a usable struct that we can manipulate
	var index IndexJSON
	byteValue, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("unable to read the contents of the file (%s) so we can add an annotation: %w", indexPath, err)
	}
	if err = json.Unmarshal(byteValue, &index); err != nil {
		return fmt.Errorf("unable to process the contents of the file (%s): %w", indexPath, err)
	}

	// Loop through the manifests and add the appropriate OCI Base Image Name Annotation
	for idx, manifest := range index.Manifests {
		if manifest.Annotations == nil {
			manifest.Annotations = make(map[string]string)
		}

		var baseImageName string

		for tag, digest := range tagToDigest {
			if digest == manifest.Digest {
				baseImageName = tag
			}
		}

		if baseImageName != "" {
			manifest.Annotations[ocispec.AnnotationBaseImageName] = baseImageName
			index.Manifests[idx] = manifest
			delete(tagToDigest, baseImageName)
		}
	}

	// Write the file back to the package
	indexJSONBytes, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, indexJSONBytes, 0600)
}
