// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2/content/file"
)

type ImageVolume struct {
	Layers      []ImageVolumeLayer
	Compression ImageVolumeCompression
	image       ocispec.Image
	tmp         string
	store       *file.Store
}

func (iv *ImageVolume) Clean() error {
	return errors.Join(iv.store.Close(), os.RemoveAll(iv.tmp))
}

func (iv *ImageVolume) AddDirectory(ctx context.Context, directory string) (err error) {
	if iv.tmp == "" {
		iv.tmp, err = utils.MakeTempDir("/tmp")
		if err != nil {
			return err
		}
	}
	if iv.store == nil {
		iv.store, err = file.New(iv.tmp)
		if err != nil {
			return err
		}
	}
	iv.image = ocispec.Image{
		Created: &static,
		History: []ocispec.History{},
		RootFS: ocispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{},
		},
	}

	logger.From(ctx).Debug("going to walk", "directory", directory)

	err = filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		layer := ImageVolumeLayer{
			Folder: directory,
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		err = layer.AddFile(path, info)
		if err != nil {
			return err
		}
		fileName, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		iv.Layers = append(iv.Layers, layer)
		iv.image.History = append(iv.image.History, ocispec.History{
			Created:   &static,
			Comment:   "dev.zarf.image.volume.v0",
			CreatedBy: fmt.Sprintf("ADD %s /", fileName),
		})

		return nil
	})

	return nil
}
