// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"bytes"
	"context"
	"fmt"

	"github.com/defenseunicorns/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// CopyPackage copies a zarf package from one OCI registry to another
func CopyPackage(ctx context.Context, src *Remote, dst *Remote, concurrency int) error {

	srcManifest, err := src.FetchRoot(ctx)
	if err != nil {
		return err
	}
	layers := append(srcManifest.Layers, srcManifest.Config)
	size := oci.SumDescsSize(layers)

	title := fmt.Sprintf("[0/%d] layers copied", len(layers))
	progressBar := message.NewProgressBar(size, title)
	defer progressBar.Stop()

	if err := oci.Copy(ctx, src.OrasRemote, dst.OrasRemote, nil, concurrency, progressBar); err != nil {
		return err
	}
	progressBar.Successf("Copied %s", src.Repo().Reference)

	srcRoot, err := src.ResolveRoot(ctx)
	if err != nil {
		return err
	}

	b, err := srcManifest.MarshalJSON()
	if err != nil {
		return err
	}
	expected := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, b)

	if err := dst.Repo().Manifests().PushReference(ctx, expected, bytes.NewReader(b), srcRoot.Digest.String()); err != nil {
		return err
	}

	tag := src.Repo().Reference.Reference
	if err := dst.UpdateIndex(ctx, tag, expected); err != nil {
		return err
	}

	src.Log().Info(fmt.Sprintf("Published %s to %s", src.Repo().Reference, dst.Repo().Reference))
	return nil
}
