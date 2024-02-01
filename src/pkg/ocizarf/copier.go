// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package ocizarf contains functions for interacting with Zarf packages stored in OCI registries.
package ocizarf

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// CopyPackage copies a zarf package from one OCI registry to another
func CopyPackage(ctx context.Context, src *ZarfOrasRemote, dst *ZarfOrasRemote, include func(d ocispec.Descriptor) bool, concurrency int) error {

	srcRoot, err := src.FetchRoot(ctx)
	if err != nil {
		return err
	}
	layers := srcRoot.GetLayers(include)
	size := oci.SumDescsSize(srcRoot.Layers)

	title := fmt.Sprintf("[0/%d] layers copied", len(layers))
	progressBar := message.NewProgressBar(size, title)
	defer progressBar.Finish(err, "Copied %s", src.Repo().Reference)

	return oci.Copy(ctx, src.OrasRemote, dst.OrasRemote, include, concurrency, progressBar)
}
