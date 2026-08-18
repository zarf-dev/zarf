// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package image is tooling for creating a multilayered container usable as an image volume mount
// for both containerd and cri-o
package image

import (
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2/content/file"
)

// NewImageVolume creates an ImageVolume backed by an OCI store at ociDir,
// with a fresh temp workspace and a config stub for the given os/arch.
func NewImageVolume(ociDir, os, arch string) (*ImageVolume, error) {
	store, err := file.New(ociDir)
	if err != nil {
		return nil, err
	}

	tmpDir, err := utils.MakeTempDir("/tmp")
	if err != nil {
		return nil, err
	}

	return &ImageVolume{
		Compression: ImageVolumeCompressionUncompressed,
		store:       store,
		tmp:         tmpDir,
		layers:      []ocispec.Descriptor{},
		config: ocispec.Image{
			Platform: ocispec.Platform{
				OS:           os,
				Architecture: arch,
			},
			Created: &static,
			History: []ocispec.History{},
			RootFS: ocispec.RootFS{
				Type:    "layers",
				DiffIDs: []digest.Digest{},
			},
		},
	}, nil
}
