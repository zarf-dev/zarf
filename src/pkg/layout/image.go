// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package layout contains functions for interacting with Zarf's package layout on disk.
package layout

import (
	"os"
	"path/filepath"

	"slices"

	"github.com/defenseunicorns/pkg/helpers"
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// Images contains paths for images.
type Images struct {
	Base      string
	Index     string
	OCILayout string
	Blobs     []string
}

// AddBlob adds a blob to the Images struct.
func (i *Images) AddBlob(blob string) {
	if len(blob) != 64 {
		return
	}
	layerPath := filepath.Join(i.Base, "blobs", "sha256")
	abs := filepath.Join(layerPath, blob)
	absSha, err := helpers.GetSHA256OfFile(abs)
	if err != nil {
		return
	}
	newPath := filepath.Join(layerPath, absSha)
	if absSha != blob {
		os.Rename(abs, newPath)
	}
	if !slices.Contains(i.Blobs, newPath) {
		i.Blobs = append(i.Blobs, newPath)
	}
}

// AddV1Image adds a v1.Image to the Images struct.
func (i *Images) AddV1Image(img v1.Image) error {
	layers, err := img.Layers()
	if err != nil {
		return err
	}
	for _, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return err
		}
		i.AddBlob(digest.Hex)
	}
	imgCfgSha, err := img.ConfigName()
	if err != nil {
		return err
	}
	i.AddBlob(imgCfgSha.Hex)
	manifestSha, err := img.Digest()
	if err != nil {
		return err
	}
	i.AddBlob(manifestSha.Hex)

	return nil
}
