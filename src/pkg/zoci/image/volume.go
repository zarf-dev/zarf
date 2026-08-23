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
	"sync"
	"sync/atomic"
	"time"

	ctdarchive "github.com/containerd/containerd/v2/core/images/archive"
	"github.com/klauspost/compress/zstd"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"

	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/zoci/archive"
)

// Volume builds an OCI image layer-by-layer from files on disk,
// pushing each layer to an underlying OCI store and tracking config/history.
type Volume struct {
	// Compression selects the tar compression format used for layers pushed
	// via AddFile/AddDirectory. The zero value behaves as
	// VolumeCompressionUncompressed.
	Compression VolumeCompression
	// MaxLayers caps the number of layers AddDirectory will produce: once
	// there are more files than MaxLayers, files are batched several-per-
	// layer to stay within it. AddFile/AddFiles called directly still fail
	// once the cap is reached, since there's no further file to batch with.
	// New sets it to DefaultMaxLayers; set to 0 to disable the cap.
	MaxLayers uint8
	// Annotations are set on the manifest AddDirectory packs, per the OCI
	// image-spec annotations rules (opaque string key/value metadata; see
	// https://github.com/opencontainers/image-spec/blob/main/annotations.md,
	// e.g. the ocispec.AnnotationTitle/AnnotationCreated pre-defined keys).
	// New initializes it to an empty map; AddDirectory adds its own
	// ocispec.AnnotationCreated entry to it, so it must be non-nil before
	// AddDirectory runs on a Volume not built via New.
	Annotations map[string]string

	layers   []ocispec.Descriptor
	tmp      string
	root     string
	store    *oci.Store
	config   ocispec.Image
	manifest ocispec.Descriptor
}

// Clean removes the temp workspace used while building layers.
func (v *Volume) Clean() error {
	return os.RemoveAll(v.tmp)
}

// Store returns the underlying OCI store.
func (v *Volume) Store() *oci.Store {
	return v.store
}

// Archive returns a read-only content.Provider backed by the OCI store's
// on-disk blobs, suitable for handing off to containerd/cri-o mount tooling.
func (v *Volume) Archive() *archive.OCIStore {
	return &archive.OCIStore{Root: v.root, Source: v.store}
}

// AddFile tars a single file, compresses it per v.Compression, pushes the
// result to the store as a layer, and records it in the image's history and
// diff IDs. path must be inside dir.
//
// The layer descriptor's digest identifies the pushed (possibly compressed)
// blob, while the diff ID recorded in RootFS.DiffIDs always identifies the
// uncompressed tar content, independent of v.Compression.
func (v *Volume) AddFile(ctx context.Context, dir, path string) (ocispec.Descriptor, error) {
	return v.AddFiles(ctx, dir, []string{path})
}

// AddFiles tars every file in paths into a single tar stream, compresses it
// per v.Compression, pushes the result to the store as one layer, and
// records it in the image's history and diff IDs. Each path must be inside
// dir. AddDirectory calls this with more than one path per layer to keep
// the total layer count within MaxLayers.
//
// The layer descriptor's digest identifies the pushed (possibly compressed)
// blob, while the diff ID recorded in RootFS.DiffIDs always identifies the
// uncompressed tar content, independent of v.Compression.
func (v *Volume) AddFiles(ctx context.Context, dir string, paths []string) (_ ocispec.Descriptor, err error) {
	if len(paths) == 0 {
		return ocispec.Descriptor{}, fmt.Errorf("no files to add")
	}
	if v.MaxLayers > 0 && len(v.layers) >= int(v.MaxLayers) {
		return ocispec.Descriptor{}, fmt.Errorf("%w: max %d, adding %d more file(s) would exceed it", ErrTooManyLayers, v.MaxLayers, len(paths))
	}

	diffID, tarPath, tarSize, fileNames, err := v.generateDiffID(dir, paths, len(v.layers))
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	mediaType, blobPath, blobDigest, blobSize, err := v.compressLayer(tarPath, diffID, tarSize)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	blob, err := os.Open(blobPath)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer func() { err = errors.Join(err, blob.Close()) }()

	// Join every file name with a blank line: readable for a handful of
	// files, and a single file collapses to its bare name unchanged.
	title := strings.Join(fileNames, "\n")

	layer := ocispec.Descriptor{
		MediaType: mediaType,
		Digest:    blobDigest,
		Size:      blobSize,
		Annotations: map[string]string{
			ocispec.AnnotationTitle:   title,
			ocispec.AnnotationCreated: format,
		},
	}

	if err := v.store.Push(ctx, layer, blob); err != nil {
		return ocispec.Descriptor{}, err
	}

	logger.From(ctx).Debug("added image volume layer",
		"files", len(fileNames),
		"title", title,
		"mediaType", mediaType,
		"digest", blobDigest,
		"diffId", diffID,
		"size", blobSize,
		"uncompressedSize", tarSize,
		"compression", v.Compression,
	)

	v.config.History = append(v.config.History, ocispec.History{
		Created:   &static,
		Comment:   "dev.zarf.zoci.volume.v0",
		CreatedBy: fmt.Sprintf("ADD %s /", title),
	})
	v.config.RootFS.DiffIDs = append(v.config.RootFS.DiffIDs, diffID)
	v.layers = append(v.layers, layer)

	return layer, nil
}

// addDirectoryLogInterval is how often AddDirectory reports progress while
// pushing layers for a large directory tree.
const addDirectoryLogInterval = 2 * time.Second

// AddDirectory walks folder and adds its files as layers via AddFiles. When
// MaxLayers is set and folder holds more files than that, files are batched
// several-per-layer so the resulting image stays within MaxLayers, rather
// than failing once there are more files than layers available.
func (v *Volume) AddDirectory(ctx context.Context, folder, ref string) error {
	l := logger.From(ctx)
	start := time.Now()
	l.Info("building image volume", "path", folder, "ref", ref, "compression", v.Compression)

	var files []string
	if err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return err
	}

	batchSize := 1
	if v.MaxLayers > 0 && len(files) > int(v.MaxLayers) {
		maxLayers := int(v.MaxLayers)
		batchSize = (len(files) + maxLayers - 1) / maxLayers
		l.Debug("batching image volume layers to fit MaxLayers", "files", len(files), "maxLayers", v.MaxLayers, "filesPerLayer", batchSize)
	}

	var added atomic.Int64
	stopTicker := make(chan struct{})
	var tickerWG sync.WaitGroup
	tickerWG.Go(func() {
		ticker := time.NewTicker(addDirectoryLogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.Info("adding image volume layers", "count", added.Load(), "path", folder)
			case <-stopTicker:
				return
			}
		}
	})
	defer func() {
		close(stopTicker)
		tickerWG.Wait()
	}()

	for i := 0; i < len(files); i += batchSize {
		end := min(i+batchSize, len(files))
		if _, err := v.AddFiles(ctx, folder, files[i:end]); err != nil {
			return err
		}
		added.Add(int64(end - i))
	}

	l.Debug("pushed image volume layers", "count", len(v.layers), "path", folder)

	configBytes, err := json.Marshal(v.config)
	if err != nil {
		return err
	}
	configDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageConfig, configBytes)
	err = v.store.Push(ctx, configDesc, bytes.NewBuffer(configBytes))
	if err != nil {
		return err
	}
	l.Debug("pushed image volume config", "digest", configDesc.Digest, "size", configDesc.Size)

	v.Annotations[ocispec.AnnotationCreated] = format

	manifestDesc, err := oras.PackManifest(
		ctx,
		v.store,
		oras.PackManifestVersion1_1,
		"application/vnd.oci.image.manifest.v1+json",
		oras.PackManifestOptions{
			Layers:              v.layers,
			ConfigDescriptor:    &configDesc,
			ManifestAnnotations: v.Annotations,
		},
	)
	if err != nil {
		return err
	}

	if err := v.store.Tag(ctx, manifestDesc, ref); err != nil {
		return err
	}
	v.manifest = manifestDesc

	l.Info("built image volume", "ref", ref, "digest", manifestDesc.Digest,
		"layers", len(v.layers), "duration", time.Since(start).Round(time.Millisecond))
	return nil
}

// WriteTar streams the image built by AddDirectory to w as a Docker/OCI
// compatible tar archive (the same layout `docker save`/`docker load`
// produce), tagging the exported manifest with ref. AddDirectory must be
// called first.
func (v *Volume) WriteTar(ctx context.Context, ref string, w io.Writer) error {
	return ctdarchive.Export(ctx, v.Archive(), w, ctdarchive.WithManifest(v.manifest, ref))
}

// writeTarFile writes file into tw as a single tar entry named rel.
func writeTarFile(tw *tar.Writer, rel, file string) (err error) {
	info, err := os.Stat(file)
	if err != nil {
		return err
	}

	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = rel
	// Pin ModTime to the same fixed timestamp used elsewhere in this package
	// (see time.go) rather than the file's real mtime: leaving the real
	// mtime in the header makes the tar bytes - and thus the diff ID -
	// depend on exactly when the file was written to disk, which is neither
	// reproducible nor stable across runs of the same content.
	hdr.ModTime = static
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	src, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, src.Close()) }()

	_, err = io.Copy(tw, src)
	return err
}

// generateDiffID tars every file in paths (given relative to dir) into a
// single tar stream in the builder's workspace and returns the digest and
// size of that stream, computed in a single pass while it is written to
// disk, along with each file's tar entry name in the same order as paths.
// batchIndex distinguishes the temp file from other layers' temp files.
func (v *Volume) generateDiffID(dir string, paths []string, batchIndex int) (dig digest.Digest, filePath string, size int64, fileNames []string, err error) {
	temp := filepath.Join(v.tmp, fmt.Sprintf("layer-%d.tar", batchIndex))
	out, err := os.Create(temp)
	if err != nil {
		return "", "", 0, nil, err
	}
	defer func() { err = errors.Join(err, out.Close()) }()

	digester := digest.Canonical.Digester()
	tw := tar.NewWriter(io.MultiWriter(out, digester.Hash()))

	fileNames = make([]string, 0, len(paths))
	for _, path := range paths {
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return "", temp, 0, nil, relErr
		}
		rel = filepath.ToSlash(rel)

		if err := writeTarFile(tw, rel, path); err != nil {
			return "", temp, 0, nil, err
		}
		fileNames = append(fileNames, rel)
	}
	if err := tw.Close(); err != nil {
		return "", temp, 0, nil, err
	}

	fi, err := out.Stat()
	if err != nil {
		return "", temp, 0, nil, err
	}

	return digester.Digest(), temp, fi.Size(), fileNames, nil
}

// compressLayer produces the on-disk blob that will be pushed to the store
// for a tarred file, applying v.Compression. tarDigest and tarSize describe
// the uncompressed tar at tarPath (as returned by generateDiffID); for
// VolumeCompressionUncompressed they are returned unchanged alongside
// tarPath, since the blob is the tar itself.
func (v *Volume) compressLayer(tarPath string, tarDigest digest.Digest, tarSize int64) (mediaType, blobPath string, dgst digest.Digest, size int64, err error) {
	switch v.Compression {
	case VolumeCompressionUncompressed, "":
		return ocispec.MediaTypeImageLayer, tarPath, tarDigest, tarSize, nil
	case VolumeCompressionGzip:
		blobPath, dgst, size, err = v.compressToFile(tarPath, func(w io.Writer) (io.WriteCloser, error) {
			return gzip.NewWriter(w), nil
		})
		return ocispec.MediaTypeImageLayerGzip, blobPath, dgst, size, err
	case VolumeCompressionZstd:
		blobPath, dgst, size, err = v.compressToFile(tarPath, func(w io.Writer) (io.WriteCloser, error) {
			return zstd.NewWriter(w)
		})
		return ocispec.MediaTypeImageLayerZstd, blobPath, dgst, size, err
	default:
		return "", "", "", 0, fmt.Errorf("unsupported image volume compression: %q", v.Compression)
	}
}

// compressToFile streams srcPath through the writer produced by
// newCompressor into a new file in the builder's workspace, computing the
// digest and size of the compressed output in a single pass.
func (v *Volume) compressToFile(srcPath string, newCompressor func(io.Writer) (io.WriteCloser, error)) (path string, dgst digest.Digest, size int64, err error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", "", 0, err
	}
	defer func() { err = errors.Join(err, src.Close()) }()

	out, err := os.CreateTemp(v.tmp, "*.layer")
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
