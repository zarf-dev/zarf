// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3"
)

// CopyToTarget copies the given layers from the remote repository to the given target
func (o *OrasRemote) CopyToTarget(ctx context.Context, layers []ocispec.Descriptor, target oras.Target, copyOpts oras.CopyOptions) error {
	layerDigests := make(map[string]struct{}, len(layers))
	for _, layer := range layers {
		if len(layer.Digest.String()) > 0 {
			layerDigests[layer.Digest.Encoded()] = struct{}{}
		}
	}
	preCopy := copyOpts.PreCopy
	copyOpts.PreCopy = func(ctx context.Context, desc ocispec.Descriptor) error {
		if preCopy != nil {
			if err := preCopy(ctx, desc); err != nil {
				return err
			}
		}

		if _, found := layerDigests[desc.Digest.Encoded()]; found {
			return nil
		}

		return oras.SkipNode
	}

	_, err := oras.Copy(ctx, o.src(), o.reference.String(), target, o.reference.String(), copyOpts)
	if err != nil {
		return err
	}
	return nil
}
