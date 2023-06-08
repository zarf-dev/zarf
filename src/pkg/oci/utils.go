// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	zarfconfig "github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry"
)

// ReferenceFromMetadata returns a reference for the given metadata.
//
// prepending the provided prefix
//
// appending the provided suffix to the version
func ReferenceFromMetadata(prefix string, metadata *types.ZarfMetadata, suffix string) (*registry.Reference, error) {
	ver := metadata.Version
	if len(ver) == 0 {
		return nil, errors.New("version is required for publishing")
	}

	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}

	format := "%s%s:%s-%s"

	raw := fmt.Sprintf(format, prefix, metadata.Name, ver, suffix)

	ref, err := registry.ParseReference(raw)
	if err != nil {
		return nil, err
	}

	return &ref, nil
}

// FetchRoot fetches the root manifest from the remote repository.
func (o *OrasRemote) FetchRoot() (*ZarfOCIManifest, error) {
	// get the manifest descriptor
	descriptor, err := o.Resolve(o.Context, o.Reference.Reference)
	if err != nil {
		return nil, err
	}

	// get the manifest itself
	bytes, err := o.FetchLayer(descriptor)
	if err != nil {
		return nil, err
	}
	manifest := ocispec.Manifest{}

	if err = json.Unmarshal(bytes, &manifest); err != nil {
		return nil, err
	}
	return NewZarfOCIManifest(&manifest), nil
}

// FetchManifest fetches the manifest with the given descriptor from the remote repository.
func (o *OrasRemote) FetchManifest(desc ocispec.Descriptor) (manifest *ZarfOCIManifest, err error) {
	bytes, err := o.FetchLayer(desc)
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(bytes, &manifest)
	if err != nil {
		return manifest, err
	}
	return manifest, nil
}

// FetchLayer fetches the layer with the given descriptor from the remote repository.
func (o *OrasRemote) FetchLayer(desc ocispec.Descriptor) (bytes []byte, err error) {
	return content.FetchAll(o.Context, o, desc)
}

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (o *OrasRemote) FetchZarfYAML(manifest *ZarfOCIManifest) (pkg types.ZarfPackage, err error) {
	zarfYamlDescriptor := manifest.Locate(zarfconfig.ZarfYAML)
	if zarfYamlDescriptor.Digest == "" {
		return pkg, fmt.Errorf("unable to find %s in the manifest", zarfconfig.ZarfYAML)
	}
	zarfYamlBytes, err := o.FetchLayer(zarfYamlDescriptor)
	if err != nil {
		return pkg, err
	}
	err = goyaml.Unmarshal(zarfYamlBytes, &pkg)
	if err != nil {
		return pkg, err
	}
	return pkg, nil
}

// FetchImagesIndex fetches the images/index.json file from the remote repository.
func (o *OrasRemote) FetchImagesIndex(manifest *ZarfOCIManifest) (index *ocispec.Index, err error) {
	indexDescriptor := manifest.Locate(manifest.indexPath)
	indexBytes, err := o.FetchLayer(indexDescriptor)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(indexBytes, &index)
	if err != nil {
		return nil, err
	}
	return index, nil
}

// printLayerSuccess prints a success message to the console when a layer has been successfully published/pulled to/from a registry.
func (o *OrasRemote) printLayerSuccess(_ context.Context, desc ocispec.Descriptor) error {
	title := desc.Annotations[ocispec.AnnotationTitle]
	var format string
	if title != "" {
		format = fmt.Sprintf("%s %s", desc.Digest.Encoded()[:12], utils.First30last30(title))
	} else {
		format = fmt.Sprintf("%s [%s]", desc.Digest.Encoded()[:12], desc.MediaType)
	}
	message.Successf(format)
	return nil
}

func (o *OrasRemote) isEmptyDescriptor(desc ocispec.Descriptor) bool {
	return desc.Digest == "" && desc.Size == 0
}
