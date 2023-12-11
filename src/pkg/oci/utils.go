// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

// ReferenceFromMetadata returns a reference for the given metadata.
//
// prepending the provided prefix
func ReferenceFromMetadata(registryLocation string, metadata *types.ZarfMetadata) (string, error) {
	ver := metadata.Version
	if len(ver) == 0 {
		return "", errors.New("version is required for publishing")
	}

	if !strings.HasSuffix(registryLocation, "/") {
		registryLocation = registryLocation + "/"
	}
	registryLocation = strings.TrimPrefix(registryLocation, helpers.OCIURLPrefix)

	format := "%s%s:%s"
	raw := fmt.Sprintf(format, registryLocation, metadata.Name, ver)

	message.Debug("Raw OCI reference from metadata:", raw)

	ref, err := registry.ParseReference(raw)
	if err != nil {
		return "", err
	}

	return ref.String(), nil
}

// printLayerSkipped prints a debug message when a layer has been successfully skipped.
func (o *OrasRemote) printLayerSkipped(_ context.Context, desc ocispec.Descriptor) error {
	return o.printLayer(desc, "skipped")
}

// printLayerCopied prints a debug message when a layer has been successfully copied to/from a registry.
func (o *OrasRemote) printLayerCopied(_ context.Context, desc ocispec.Descriptor) error {
	return o.printLayer(desc, "copied")
}

// printLayer prints a debug message when a layer has been successfully published/pulled to/from a registry.
func (o *OrasRemote) printLayer(desc ocispec.Descriptor, suffix string) error {
	title := desc.Annotations[ocispec.AnnotationTitle]
	var layerInfo string
	if title != "" {
		layerInfo = fmt.Sprintf("%s %s", desc.Digest.Encoded()[:12], utils.First30last30(title))
	} else {
		layerInfo = fmt.Sprintf("%s [%s]", desc.Digest.Encoded()[:12], desc.MediaType)
	}
	message.Debugf("%s (%s)", layerInfo, suffix)
	return nil
}

// IsEmptyDescriptor returns true if the given descriptor is empty.
func IsEmptyDescriptor(desc ocispec.Descriptor) bool {
	return desc.Digest == "" && desc.Size == 0
}

// ValidateReference validates the given url is a valid OCI reference.
func ValidateReference(url string) error {
	if !strings.HasPrefix(url, helpers.OCIURLPrefix) {
		return fmt.Errorf("oci url reference must begin with %s", helpers.OCIURLPrefix)
	}
	sansPrefix := strings.TrimPrefix(url, helpers.OCIURLPrefix)
	_, err := registry.ParseReference(sansPrefix)
	return err
}

// RemoveDuplicateDescriptors removes duplicate descriptors from the given list.
func RemoveDuplicateDescriptors(descriptors []ocispec.Descriptor) []ocispec.Descriptor {
	keys := make(map[string]bool)
	list := []ocispec.Descriptor{}
	for _, entry := range descriptors {
		if IsEmptyDescriptor(entry) {
			continue
		}
		if _, value := keys[entry.Digest.Encoded()]; !value {
			keys[entry.Digest.Encoded()] = true
			list = append(list, entry)
		}
	}
	return list
}

// GetInitPackageURL returns the URL for the init package for the given version.
func GetInitPackageURL(version string) string {
	return fmt.Sprintf("ghcr.io/defenseunicorns/packages/init:%s", version)
}
