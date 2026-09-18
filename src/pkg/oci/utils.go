// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"context"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

// IsEmptyDescriptor returns true if the given descriptor is empty.
func IsEmptyDescriptor(descriptor ocispec.Descriptor) bool {
	return descriptor.Digest == "" && descriptor.Size == 0
}

// RemoveDuplicateDescriptors removes duplicate descriptors from the given list.
func RemoveDuplicateDescriptors(descriptors []ocispec.Descriptor) []ocispec.Descriptor {
	seen := make(map[string]bool, len(descriptors))
	result := make([]ocispec.Descriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if IsEmptyDescriptor(descriptor) || seen[descriptor.Digest.Encoded()] {
			continue
		}
		seen[descriptor.Digest.Encoded()] = true
		result = append(result, descriptor)
	}
	return result
}

// SumDescsSize returns the size of all the descriptors added together
func SumDescsSize(descriptors []ocispec.Descriptor) int64 {
	var total int64
	for _, descriptor := range descriptors {
		total += descriptor.Size
	}
	return total
}

// GetDefaultCopyOpts returns the default copy options
func (remote *OrasRemote) GetDefaultCopyOpts() oras.CopyOptions {
	options := oras.DefaultCopyOptions
	options.OnCopySkipped = func(_ context.Context, descriptor ocispec.Descriptor) error {
		remote.logLayer(descriptor, "skipped")
		return nil
	}
	options.PostCopy = func(_ context.Context, descriptor ocispec.Descriptor) error {
		remote.logLayer(descriptor, "copied")
		return nil
	}
	return options
}

// logLayer prints a debug message when a layer has been successfully published/pulled to/from a registry.
func (remote *OrasRemote) logLayer(descriptor ocispec.Descriptor, operation string) {
	title := descriptor.Annotations[ocispec.AnnotationTitle]
	if title == "" {
		title = fmt.Sprintf("[%s]", descriptor.MediaType)
	}
	digest := descriptor.Digest.Encoded()
	if len(digest) > 12 {
		digest = digest[:12]
	}
	remote.log.Debug("operation successful", "layer", fmt.Sprintf("%s %s", digest, title), "operation", operation)
}
