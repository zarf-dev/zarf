// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/file"
)

// ImageVolumeCompression names the tar compression format used for layers.
type ImageVolumeCompression string

// These are the 3 valid tar formats
const (
	// ImageVolumeCompressionGzip is the gzip compression format.
	ImageVolumeCompressionGzip ImageVolumeCompression = "gzip"
	// ImageVolumeCompressionZstd is the zstd compression format.
	ImageVolumeCompressionZstd ImageVolumeCompression = "zstd"
	// ImageVolumeCompressionUncompressed is the uncompressed compression format.
	ImageVolumeCompressionUncompressed ImageVolumeCompression = "uncompressed"
)

// ImageVolume builds an OCI image layer-by-layer from files on disk,
// pushing each layer to an underlying OCI store and tracking config/history.
type ImageVolume struct {
	Compression ImageVolumeCompression
	layers      []ocispec.Descriptor
	tmp         string
	store       *file.Store
	config      ocispec.Image
}

// Clean closes the underlying OCI store and removes the temp workspace.
func (iv *ImageVolume) Clean() error {
	return errors.Join(iv.store.Close(), os.RemoveAll(iv.tmp))
}

// Store returns the underlying OCI file store.
func (iv *ImageVolume) Store() *file.Store {
	return iv.store
}

// AddFile tars a single file, pushes it to the store as a layer, and
// records it in the image's history and diff IDs. path must be inside dir.
func (iv *ImageVolume) AddFile(ctx context.Context, dir, path string) (_ ocispec.Descriptor, err error) {
	dgst, tarPath, size, err := iv.generateDiffID(dir, path)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	tgz, err := os.Open(tarPath)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer func() { err = errors.Join(err, tgz.Close()) }()

	fileName, err := filepath.Rel(dir, path)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	layer := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayer,
		Digest:    dgst,
		Size:      size,
		Annotations: map[string]string{
			ocispec.AnnotationTitle:   fileName,
			ocispec.AnnotationCreated: format,
		},
	}

	fmt.Printf("pushing %s (%s, %d bytes)\n", fileName, dgst, size)
	if err := iv.store.Push(ctx, layer, tgz); err != nil {
		return ocispec.Descriptor{}, err
	}

	iv.config.History = append(iv.config.History, ocispec.History{
		Created:   &static,
		Comment:   "dev.zarf.image.volume.v0",
		CreatedBy: fmt.Sprintf("ADD %s /", fileName),
	})
	iv.config.RootFS.DiffIDs = append(iv.config.RootFS.DiffIDs, dgst)
	iv.layers = append(iv.layers, layer)

	return layer, nil
}

// AddDirectory walks folder and adds each regular file as a layer via AddFile.
func (iv *ImageVolume) AddDirectory(ctx context.Context, folder string) error {
	fmt.Printf("walking %s\n", folder)

	err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		_, err = iv.AddFile(ctx, folder, path)
		return err
	})
	if err != nil {
		return err
	}

	fmt.Printf("pushed %d files from %s\n", len(iv.layers), folder)

	return nil
}

// generateDiffID tars file into the builder's workspace and returns the
// digest and size of the resulting tar stream, computed in a single pass
// while it is written to disk.
func (iv *ImageVolume) generateDiffID(dir, file string) (dig digest.Digest, filePath string, size int64, err error) {
	info, err := os.Stat(file)
	if err != nil {
		return "", "", 0, err
	}

	rel := strings.TrimPrefix(file, dir+"/")
	temp := filepath.Join(iv.tmp, strings.ReplaceAll(rel, string(filepath.Separator), "_")+".tar")
	out, err := os.Create(temp)
	if err != nil {
		return "", "", 0, err
	}
	defer func() { err = errors.Join(err, out.Close()) }()

	digester := digest.Canonical.Digester()
	tw := tar.NewWriter(io.MultiWriter(out, digester.Hash()))

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return "", temp, 0, err
	}
	hdr.Name = rel
	if err := tw.WriteHeader(hdr); err != nil {
		return "", temp, 0, err
	}

	src, err := os.Open(file)
	if err != nil {
		return "", temp, 0, err
	}
	defer func() { err = errors.Join(err, src.Close()) }()

	if _, err := io.Copy(tw, src); err != nil {
		return "", temp, 0, err
	}
	if err := tw.Close(); err != nil {
		return "", temp, 0, err
	}

	fi, err := out.Stat()
	if err != nil {
		return "", temp, 0, err
	}

	return digester.Digest(), temp, fi.Size(), nil
}
