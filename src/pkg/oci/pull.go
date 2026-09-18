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
func (remote *OrasRemote) CopyToTarget(ctx context.Context, layers []ocispec.Descriptor, target oras.Target, options oras.CopyOptions) error {
	digests := make([]string, 0, len(layers))
	for _, layer := range layers {
		if layer.Digest != "" {
			digests = append(digests, layer.Digest.Encoded())
		}
	}
	preCopy := options.PreCopy
	options.PreCopy = func(ctx context.Context, descriptor ocispec.Descriptor) error {
		if preCopy != nil {
			if err := preCopy(ctx, descriptor); err != nil {
				return err
			}
		}
		if slices.Contains(digests, descriptor.Digest.Encoded()) {
			return nil
		}
		return oras.SkipNode
	}
	_, err := oras.Copy(ctx, remote.source(), remote.repo.Reference.String(), target, remote.repo.Reference.String(), options)
	return err
}
