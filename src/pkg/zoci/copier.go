// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"bytes"
	"context"
	"fmt"
	"github.com/zarf-dev/zarf/src/pkg/logger"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

// CopyPackage copies a zarf package from one OCI registry to another
func CopyPackage(ctx context.Context, src *Remote, dst *Remote, concurrency int) (err error) {
	l := logger.From(ctx)
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	srcManifest, err := src.FetchRoot(ctx)
	if err != nil {
		return err
	}
	l.Info("copying package", "src", src.String(), "dst", dst.String())
	if err := oci.Copy(ctx, src.OrasRemote, dst.OrasRemote, nil, concurrency, nil); err != nil {
		return err
	}

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
