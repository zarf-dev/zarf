// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package image

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"

	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// ImageVolume builds an OCI image layer-by-layer from files on disk,
// pushing each layer to an underlying OCI store and tracking config/history.
type ImageVolume struct {
	// Compression selects the tar compression format used for layers pushed
	// via AddFile/AddDirectory. The zero value behaves as
	// ImageVolumeCompressionUncompressed.
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

// AddFile tars a single file, compresses it per iv.Compression, pushes the
// result to the store as a layer, and records it in the image's history and
// diff IDs. path must be inside dir.
//
// The layer descriptor's digest identifies the pushed (possibly compressed)
// blob, while the diff ID recorded in RootFS.DiffIDs always identifies the
// uncompressed tar content, independent of iv.Compression.
func (iv *ImageVolume) AddFile(ctx context.Context, dir, path string) (_ ocispec.Descriptor, err error) {
	fileName, err := filepath.Rel(dir, path)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	diffID, tarPath, tarSize, err := iv.generateDiffID(fileName, path)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	mediaType, blobPath, blobDigest, blobSize, err := iv.compressLayer(tarPath, diffID, tarSize)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	blob, err := os.Open(blobPath)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer func() { err = errors.Join(err, blob.Close()) }()

	layer := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    blobDigest,
		Size:      blobSize,
		Annotations: map[string]string{
			ocispec.AnnotationTitle:   fileName,
			ocispec.AnnotationCreated: format,
		},
	}

	logger.From(ctx).Debug("pushing image volume layer", "file", fileName, "digest", blobDigest, "size", blobSize)
	if err := iv.store.Push(ctx, layer, blob); err != nil {
		return ocispec.Descriptor{}, err
	}

	iv.config.History = append(iv.config.History, ocispec.History{
		Created:   &static,
		Comment:   "dev.zarf.zoci.volume.v0",
		CreatedBy: fmt.Sprintf("ADD %s /", fileName),
	})
	iv.config.RootFS.DiffIDs = append(iv.config.RootFS.DiffIDs, diffID)
	iv.layers = append(iv.layers, layer)

	return layer, nil
}

// AddDirectory walks folder and adds each regular file as a layer via AddFile.
func (iv *ImageVolume) AddDirectory(ctx context.Context, folder, ref string) error {
	l := logger.From(ctx)
	l.Debug("walking directory for image volume", "path", folder)

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

	l.Debug("pushed files for image volume", "count", len(iv.layers), "path", folder)

	configBytes, err := json.Marshal(iv.config)
	if err != nil {
		return err
	}
	configDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageConfig, configBytes)
	err = iv.store.Push(ctx, configDesc, bytes.NewBuffer(configBytes))
	if err != nil {
		return err
	}

	manifestDesc, err := oras.PackManifest(
		ctx,
		iv.store,
		oras.PackManifestVersion1_1,
		"application/vnd.oci.image.manifest.v1+json",
		oras.PackManifestOptions{
			Layers:           iv.layers,
			ConfigDescriptor: &configDesc,
			ManifestAnnotations: map[string]string{
				ocispec.AnnotationCreated: format,
			},
		},
	)
	if err != nil {
		return err
	}

	return iv.store.Tag(ctx, manifestDesc, ref)
}

// generateDiffID tars file into the builder's workspace under tar entry name
// rel and returns the digest and size of the resulting tar stream, computed
// in a single pass while it is written to disk.
func (iv *ImageVolume) generateDiffID(rel, file string) (dig digest.Digest, filePath string, size int64, err error) {
	info, err := os.Stat(file)
	if err != nil {
		return "", "", 0, err
	}

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

// compressLayer produces the on-disk blob that will be pushed to the store
// for a tarred file, applying iv.Compression. tarDigest and tarSize describe
// the uncompressed tar at tarPath (as returned by generateDiffID); for
// ImageVolumeCompressionUncompressed they are returned unchanged alongside
// tarPath, since the blob is the tar itself.
func (iv *ImageVolume) compressLayer(tarPath string, tarDigest digest.Digest, tarSize int64) (mediaType, blobPath string, dgst digest.Digest, size int64, err error) {
	switch iv.Compression {
	case ImageVolumeCompressionUncompressed, "":
		return ocispec.MediaTypeImageLayer, tarPath, tarDigest, tarSize, nil
	case ImageVolumeCompressionGzip:
		blobPath, dgst, size, err = iv.compressToFile(tarPath, func(w io.Writer) (io.WriteCloser, error) {
			return gzip.NewWriter(w), nil
		})
		return ocispec.MediaTypeImageLayerGzip, blobPath, dgst, size, err
	case ImageVolumeCompressionZstd:
		blobPath, dgst, size, err = iv.compressToFile(tarPath, func(w io.Writer) (io.WriteCloser, error) {
			return zstd.NewWriter(w)
		})
		return ocispec.MediaTypeImageLayerZstd, blobPath, dgst, size, err
	default:
		return "", "", "", 0, fmt.Errorf("unsupported image volume compression: %q", iv.Compression)
	}
}

// compressToFile streams srcPath through the writer produced by
// newCompressor into a new file in the builder's workspace, computing the
// digest and size of the compressed output in a single pass.
func (iv *ImageVolume) compressToFile(srcPath string, newCompressor func(io.Writer) (io.WriteCloser, error)) (path string, dgst digest.Digest, size int64, err error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", "", 0, err
	}
	defer func() { err = errors.Join(err, src.Close()) }()

	out, err := os.CreateTemp(iv.tmp, "*.layer")
	if err != nil {
		return "", "", 0, err
	}
	defer func() { err = errors.Join(err, out.Close()) }()

	digester := digest.Canonical.Digester()
	cw, err := newCompressor(io.MultiWriter(out, digester.Hash()))
	if err != nil {
		return "", "", 0, err
	}

	if _, err := io.Copy(cw, src); err != nil {
		return "", "", 0, err
	}
	if err := cw.Close(); err != nil {
		return "", "", 0, err
	}

	fi, err := out.Stat()
	if err != nil {
		return "", "", 0, err
	}

	return out.Name(), digester.Digest(), fi.Size(), nil
}
