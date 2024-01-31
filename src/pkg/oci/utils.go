// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"fmt"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

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

func SumLayersSize(layers []ocispec.Descriptor) int64 {
	var sum int64
	for _, layer := range layers {
		sum += layer.Size
	}
	return sum
}
