// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package oci

import (
	"context"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/defenseunicorns/pkg/helpers/v2"
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
		layerInfo = fmt.Sprintf("%s %s", desc.Digest.Encoded()[:12], helpers.First30Last30(title))
	} else {
		layerInfo = fmt.Sprintf("%s [%s]", desc.Digest.Encoded()[:12], desc.MediaType)
	}
	o.log.Debug("operation successful", "layer", layerInfo, "operation", suffix)
	return nil
}
