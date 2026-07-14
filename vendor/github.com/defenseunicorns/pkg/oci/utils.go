// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package oci

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

// IsEmptyDescriptor returns true if the given descriptor is empty.
func IsEmptyDescriptor(desc ocispec.Descriptor) bool {
	return desc.Digest == "" && desc.Size == 0
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

// SumDescsSize returns the size of all the descriptors added together
func SumDescsSize(descs []ocispec.Descriptor) int64 {
	var sum int64
	for _, layer := range descs {
		sum += layer.Size
	}
	return sum
}

// GetDefaultCopyOpts returns the default copy options
func (o *OrasRemote) GetDefaultCopyOpts() oras.CopyOptions {
	copyOpts := oras.DefaultCopyOptions
	copyOpts.OnCopySkipped = o.printLayerSkipped
	copyOpts.PostCopy = o.printLayerCopied
	return copyOpts
}
