// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config"
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
func ReferenceFromMetadata(registryLocation string, metadata *types.ZarfMetadata, suffix string) (string, error) {
	ver := metadata.Version
	if len(ver) == 0 {
		return "", errors.New("version is required for publishing")
	}

	if !strings.HasSuffix(registryLocation, "/") {
		registryLocation = registryLocation + "/"
	}
	registryLocation = strings.TrimPrefix(registryLocation, utils.OCIURLPrefix)

	format := "%s%s:%s-%s"

	raw := fmt.Sprintf(format, registryLocation, metadata.Name, ver, suffix)

	message.Debug("Raw OCI reference from metadata:", raw)

	ref, err := registry.ParseReference(raw)
	if err != nil {
		return "", err
	}

	return ref.String(), nil
}

// FetchRoot fetches the root manifest from the remote repository.
func (o *OrasRemote) FetchRoot() (*ZarfOCIManifest, error) {
	if o.root != nil {
		return o.root, nil
	}
	// get the manifest descriptor
	descriptor, err := o.repo.Resolve(o.ctx, o.repo.Reference.Reference)
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
	root := NewZarfOCIManifest(&manifest)
	o.root = root
	return o.root, nil
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
	return content.FetchAll(o.ctx, o.repo, desc)
}

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (o *OrasRemote) FetchZarfYAML(manifest *ZarfOCIManifest) (pkg types.ZarfPackage, err error) {
	return FetchYAML[types.ZarfPackage](o.FetchLayer, manifest, config.ZarfYAML)
}

// FetchImagesIndex fetches the images/index.json file from the remote repository.
func (o *OrasRemote) FetchImagesIndex(manifest *ZarfOCIManifest) (index *ocispec.Index, err error) {
	return FetchJSON[*ocispec.Index](o.FetchLayer, manifest, manifest.indexPath)
}

// FetchJSON fetches the given JSON file from the remote repository.
func FetchJSON[T any](fetcher func(desc ocispec.Descriptor) (bytes []byte, err error), manifest *ZarfOCIManifest, path string) (result T, err error) {
	descriptor := manifest.Locate(path)
	if IsEmptyDescriptor(descriptor) {
		return result, fmt.Errorf("unable to find %s in the manifest", path)
	}
	bytes, err := fetcher(descriptor)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(bytes, &result)
	if err != nil {
		return result, err
	}
	return result, nil
}

// FetchYAML fetches the given YAML file from the remote repository.
func FetchYAML[T any](fetcher func(desc ocispec.Descriptor) (bytes []byte, err error), manifest *ZarfOCIManifest, path string) (result T, err error) {
	descriptor := manifest.Locate(path)
	if IsEmptyDescriptor(descriptor) {
		return result, fmt.Errorf("unable to find %s in the manifest", path)
	}
	bytes, err := fetcher(descriptor)
	if err != nil {
		return result, err
	}
	err = goyaml.Unmarshal(bytes, &result)
	if err != nil {
		return result, err
	}
	return result, nil
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

// FileExists returns true if the given file exists in the given directory with the expected SHA.
func (o *OrasRemote) FileExists(desc ocispec.Descriptor, destinationDir string) bool {
	destinationPath := filepath.Join(destinationDir, desc.Annotations[ocispec.AnnotationTitle])
	info, err := os.Stat(destinationPath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if info.Size() != desc.Size {
		return false
	}

	actual, err := utils.GetSHA256OfFile(destinationPath)
	if err != nil {
		return false
	}
	return actual == desc.Digest.Encoded()
}

// IsEmptyDescriptor returns true if the given descriptor is empty.
func IsEmptyDescriptor(desc ocispec.Descriptor) bool {
	return desc.Digest == "" && desc.Size == 0
}

// ValidateReference validates the given url is a valid OCI reference.
func ValidateReference(url string) error {
	if !strings.HasPrefix(url, "oci://") {
		return fmt.Errorf("oci url reference must begin with oci://")
	}
	sansPrefix := strings.TrimPrefix(url, "oci://")
	_, err := registry.ParseReference(sansPrefix)
	return err
}
