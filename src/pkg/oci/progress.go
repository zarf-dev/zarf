// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

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
		layerInfo = fmt.Sprintf("%s %s", desc.Digest.Encoded()[:12], message.First30last30(title))
	} else {
		layerInfo = fmt.Sprintf("%s [%s]", desc.Digest.Encoded()[:12], desc.MediaType)
	}
	o.log("%s (%s)", layerInfo, suffix)
	return nil
}
