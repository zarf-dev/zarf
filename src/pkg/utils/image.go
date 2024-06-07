// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// LoadOCIImage returns a v1.Image with the image ref specified from a location provided, or an error if the image cannot be found.
func LoadOCIImage(imgPath string, refInfo transform.Image) (v1.Image, error) {
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

	// Search through all the manifests within this package until we find the annotation that matches our ref
	for _, manifest := range idxManifest.Manifests {
		if manifest.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Reference ||
			// A backwards compatibility shim for older Zarf versions that would leave docker.io off of image annotations
			(manifest.Annotations[ocispec.AnnotationBaseImageName] == refInfo.Path+refInfo.TagOrDigest && refInfo.Host == "docker.io") {

			// This is the image we are looking for, load it and then return
			return layoutPath.Image(manifest.Digest)
		}
	}

	return nil, fmt.Errorf("unable to find image (%s) at the path (%s)", refInfo.Reference, imgPath)
}

// AddImageNameAnnotation adds an annotation to the index.json file so that the deploying code can figure out what the image reference <-> digest shasum will be.
func AddImageNameAnnotation(ociPath string, referenceToDigest map[string]string) error {
	indexPath := filepath.Join(ociPath, "index.json")

	var index ocispec.Index
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("unable to read the contents of the file (%s) so we can add an annotation: %w", indexPath, err)
	}
	if err = json.Unmarshal(b, &index); err != nil {
		return fmt.Errorf("unable to process the contents of the file (%s): %w", indexPath, err)
	}

	// Loop through the manifests and add the appropriate OCI Base Image Name Annotation
	for idx, manifest := range index.Manifests {
		if manifest.Annotations == nil {
			manifest.Annotations = make(map[string]string)
		}

		var baseImageName string

		for reference, digest := range referenceToDigest {
			if digest == manifest.Digest.String() {
				baseImageName = reference
			}
		}

		if baseImageName != "" {
			manifest.Annotations[ocispec.AnnotationBaseImageName] = baseImageName
			index.Manifests[idx] = manifest
			delete(referenceToDigest, baseImageName)
		}
	}

	// Write the file back to the package
	b, err = json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(indexPath, b, helpers.ReadWriteUser)
}

// HasImageLayers checks if all layers in the v1.Image are known image layers.
func HasImageLayers(img v1.Image) (bool, error) {
	layers, err := img.Layers()
	if err != nil {
		return false, err
	}
	for _, layer := range layers {
		mediatype, err := layer.MediaType()
		if err != nil {
			return false, err
		}
		if !mediatype.IsLayer() {
			return false, nil
		}
	}
	return true, nil
}
