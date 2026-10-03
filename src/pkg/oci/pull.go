// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"context"
	"slices"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
)

// CopyToTarget copies the given layers from the remote repository to the given target
func (o *OrasRemote) CopyToTarget(ctx context.Context, layers []ocispec.Descriptor, target oras.Target, copyOpts oras.CopyOptions) error {
	shas := []string{}
	for _, layer := range layers {
		if len(layer.Digest.String()) > 0 {
			shas = append(shas, layer.Digest.Encoded())
		}
	}

	preCopy := copyOpts.PreCopy
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		if preCopy != nil {
			if err := preCopy(ctx, desc); err != nil {
				return err
			}
		}

		if slices.Contains(shas, desc.Digest.Encoded()) {
			return nil
		}

		return oras.SkipNode
	}

	_, err := oras.Copy(ctx, o.src(), o.repo.Reference.String(), target, o.repo.Reference.String(), copyOpts)
	if err != nil {
		return err
	}
	return nil
}
