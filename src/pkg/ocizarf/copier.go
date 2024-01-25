// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package ocizarf

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func CopyPackage(ctx context.Context, src *oci.OrasRemote, dst *oci.OrasRemote,
	include func(d ocispec.Descriptor) bool, concurrency int) error {

	srcRoot, err := src.FetchRoot()
	if err != nil {
		return err
	}
	layers := srcRoot.GetLayers(include)
	size := srcRoot.SumLayersSize()

	title := fmt.Sprintf("[0/%d] layers copied", len(layers))
	progressBar := message.NewProgressBar(size, title)
	defer progressBar.Finish(err, "Copied %s", src.Repo().Reference)

	if err = oci.Copy(ctx, src, dst, include, concurrency, progressBar); err != nil {
		return err
	}
	return nil
}
