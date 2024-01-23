// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"errors"
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

// ReferenceFromMetadata returns a reference for the given metadata.
func ReferenceFromMetadata(registryLocation string, metadata *types.ZarfMetadata, build *types.ZarfBuildData) (string, error) {
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

	if build != nil && build.Flavor != "" {
		raw = fmt.Sprintf("%s-%s", raw, build.Flavor)
	}

	//o.log("Raw OCI reference from metadata:", raw)

	ref, err := registry.ParseReference(raw)
	if err != nil {
		return "", err
	}

	return ref.String(), nil
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
