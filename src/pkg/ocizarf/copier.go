// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package ocizarf

import (
	"context"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func CopyPackage(ctx context.Context, src *oci.OrasRemote, dst *oci.OrasRemote, include func(d ocispec.Descriptor) bool, concurrency int) error {

	if err := oci.CopyPackage(ctx, src, dst, nil, config.CommonOptions.OCIConcurrency); err != nil {
		return err
	}
	return nil
}
