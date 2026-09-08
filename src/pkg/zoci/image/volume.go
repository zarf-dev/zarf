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
	if len(fileNames) == 0 {
		return ocispec.Descriptor{}, fmt.Errorf("no files to add: every path given was a file type that cannot be stored in an image volume")
	}

	mediaType, blobPath, blobDigest, blobSize, err := v.compressLayer(tarPath, diffID, tarSize)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	blob, err := os.Open(blobPath)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	// Drop the workspace copies once the blob is in the store. Without this
	// every layer's tar (and, when compressing, its compressed twin) stays on
	// disk for the whole build, so a large source tree needs several times its
	// own size in temp space before Clean runs. Registered before the close
	// below so it runs after it: Windows will not unlink an open file. A
	// failure here only wastes space that Clean reclaims later, so it is
	// logged rather than returned.
	defer func() {
		if rmErr := removeLayerTemps(tarPath, blobPath); rmErr != nil {
			logger.From(ctx).Debug("failed to remove image volume layer workspace files", "error", rmErr)
		}
	}()
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
			ocispec.AnnotationCreated: staticRFC3339,
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

// removeLayerTemps deletes the workspace files a single layer produced. For
// an uncompressed layer blobPath and tarPath are the same file, so only one
// delete happens. Already-missing files are not an error.
func removeLayerTemps(tarPath, blobPath string) error {
	var errs []error
	for _, path := range []string{tarPath, blobPath} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
		if blobPath == tarPath {
			break
		}
	}
	return errors.Join(errs...)
}

// addDirectoryLogInterval is how often AddDirectory reports progress while
// pushing layers for a large directory tree.
const addDirectoryLogInterval = 2 * time.Second

// AddDirectory walks folder and adds its files as layers via AddFiles. When
// MaxLayers is set and folder holds more files than the volume has layers
// left, files are batched several-per-layer so the resulting image stays
// within MaxLayers, rather than failing once there are more files than layers
// available.
//
// Symlinks are stored as symlinks and are never followed. Files that have no
// representation in an image volume (devices, sockets, FIFOs) are skipped
// with a warning. AddDirectory returns an error if folder holds no files it
// can store.
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
		// d.Type() holds only the type bits, so zero means a regular file.
		// Symlinks are kept and written as links by writeTarFile; devices,
		// sockets, and FIFOs cannot be stored in an image volume at all.
		if t := d.Type(); t != 0 && t&fs.ModeSymlink == 0 {
			l.Warn("skipping file that cannot be stored in an image volume", "path", path, "mode", t)
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no files to add: %q holds nothing that can be stored in an image volume", folder)
	}

	// Budget against the layers already on the volume, not against MaxLayers
	// outright: a Volume that has been added to before has fewer layers left
	// to spend, and batching as though it were empty would push blobs and
	// only then fail on the cap.
	batchSize := 1
	if v.MaxLayers > 0 {
		remaining := int(v.MaxLayers) - len(v.layers)
		if remaining <= 0 {
			return fmt.Errorf("%w: max %d, already at %d", ErrTooManyLayers, v.MaxLayers, len(v.layers))
		}
		if len(files) > remaining {
			batchSize = (len(files) + remaining - 1) / remaining
			l.Debug("batching image volume layers to fit MaxLayers", "files", len(files), "maxLayers", v.MaxLayers, "layersRemaining", remaining, "filesPerLayer", batchSize)
		}
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

	v.Annotations[ocispec.AnnotationCreated] = staticRFC3339

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
// called first; WriteTar returns ErrNoManifest otherwise, rather than writing
// the valid-but-imageless archive the exporter would produce from a
// zero-value manifest descriptor.
func (v *Volume) WriteTar(ctx context.Context, ref string, w io.Writer) error {
	if v.manifest.Digest == "" {
		return ErrNoManifest
	}
	return ctdarchive.Export(ctx, v.Archive(), w, ctdarchive.WithManifest(v.manifest, ref))
}

// writeTarFile writes file into tw as a single tar entry named rel and
// reports whether an entry was written.
//
// Regular files are written with their contents and symlinks are written as
// symlink entries pointing at their raw target - the link is never followed,
// so a symlink out of the source directory stays a dangling link inside the
// image instead of silently copying outside content in. Every other file
// type (directories reached through a symlink, devices, sockets, FIFOs) has
// no useful representation in an image volume and is skipped; opening a FIFO
// would otherwise block until a writer appeared.
func writeTarFile(tw *tar.Writer, rel, file string) (written bool, err error) {
	// Lstat, not Stat: Stat follows symlinks, which turns a link to a
	// directory into a directory header whose contents cannot be read, and a
	// link out of the tree into a silent copy of outside content.
	info, err := os.Lstat(file)
	if err != nil {
		return false, err
	}

	var link string
	switch {
	case info.Mode().IsRegular():
	case info.Mode()&fs.ModeSymlink != 0:
		link, err = os.Readlink(file)
		if err != nil {
			return false, err
		}
	default:
		return false, nil
	}

	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return false, err
	}
	hdr.Name = rel
	// Pin every header field that would otherwise vary with the machine and
	// moment that happen to run the build. Timestamps come from the fixed
	// value in time.go, and ownership is flattened to root: leaving the real
	// mtime or the building user's uid/gid in the header makes the tar bytes
	// - and thus the diff ID and the manifest digest - depend on who built
	// the image and when, so the same source tree would not produce the same
	// image twice. Pinning the format keeps the encoding from drifting with
	// the Go version too.
	hdr.ModTime = static
	hdr.AccessTime = time.Time{}
	hdr.ChangeTime = time.Time{}
	hdr.Uid, hdr.Gid = 0, 0
	hdr.Uname, hdr.Gname = "", ""
	hdr.Format = tar.FormatPAX

	if err := tw.WriteHeader(hdr); err != nil {
		return false, err
	}
	if link != "" {
		// A symlink entry is header-only; there is no payload to copy.
		return true, nil
	}

	src, err := os.Open(file)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, src.Close()) }()

	_, err = io.Copy(tw, src)
	return err == nil, err
}

// generateDiffID tars every file in paths (given relative to dir) into a
// single tar stream in the builder's workspace and returns the digest and
// size of that stream, computed in a single pass while it is written to
// disk, along with each file's tar entry name in the same order as paths.
// Paths that writeTarFile skips are absent from fileNames, so it can be
// shorter than paths. batchIndex distinguishes the temp file from other
// layers' temp files.
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

		written, writeErr := writeTarFile(tw, rel, path)
		if writeErr != nil {
			return "", temp, 0, nil, writeErr
		}
		if !written {
			continue
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
