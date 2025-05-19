// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package archive contains the SDK for Zarf archival and compression.
package archive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
	"github.com/zarf-dev/zarf/src/config/lang"
)

const (
	extensionTar  = ".tar"
	extensionZip  = ".zip"
	extensionGz   = ".tar.gz"
	extensionTgz  = ".tgz"
	extensionBz2  = ".tar.bz2"
	extensionTbz2 = ".tbz2"
	extensionTbz  = ".tbz"
	extensionXz   = ".tar.xz"
	extensionTxz  = ".txz"
	extensionZst  = ".tar.zst"
	extensionTzst = ".tzst"
	extensionBr   = ".tar.br"
	extensionTbr  = ".tbr"
	extensionLz4  = ".tar.lz4"
	extensionTlz4 = ".tlz4"
	extensionLzip = ".tar.lz"
	extensionMz   = ".tar.mz"
	extensionTmz  = ".tmz"
	sbomFileName  = "sbom.tar"

	dirPerm  = 0o755
	filePerm = 0o644
)

// CompressOpts is a placeholder for future optional Compress params
type CompressOpts struct{}

// Compress archives the given source files into dest, inferring the compression format by extension.
func Compress(ctx context.Context, sources []string, dest string, _ CompressOpts) (err error) {
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dest, err)
	}
	defer func() { err = errors.Join(err, out.Close()) }()

	mapping := make(map[string]string, len(sources))
	for _, src := range sources {
		mapping[src] = filepath.Base(src)
	}
	files, err := archives.FilesFromDisk(ctx, nil, mapping)
	if err != nil {
		return fmt.Errorf("failed to stat sources: %w", err)
	}

	archiver := selectArchiver(dest)
	if archiver == nil {
		return fmt.Errorf("unsupported archive extension for %q", dest)
	}
	if err := archiver.Archive(ctx, out, files); err != nil {
		return fmt.Errorf("archive failed for %q: %w", dest, err)
	}
	return nil
}

// DecompressOpts provides optional parameters for Decompress operations.
type DecompressOpts struct {
	UnarchiveAll      bool
	Files             []string
	StripComponents   int
	OverwriteExisting bool
	SkipValidation    bool
}

// Decompress extracts from sourceArchive into dst according to opts.
func Decompress(ctx context.Context, sourceArchive, dst string, opts DecompressOpts) error {
	if len(opts.Files) > 0 {
		err := extractFiltered(ctx, sourceArchive, dst, opts.Files, opts.SkipValidation)
		if err != nil {
			return fmt.Errorf("unable to extract filtered files from %q: %w", sourceArchive, err)
		}
		return nil
	}

	if opts.StripComponents > 0 || opts.OverwriteExisting {
		err := extract(ctx, sourceArchive, dst,
			stripHandler(dst, opts.StripComponents, opts.OverwriteExisting))
		if err != nil {
			return fmt.Errorf("unable to strip-extract %q: %w", sourceArchive, err)
		}
	} else {
		err := extract(ctx, sourceArchive, dst, basicHandler(dst))
		if err != nil {
			return fmt.Errorf("unable to extract %q: %w", sourceArchive, err)
		}
	}

	if opts.UnarchiveAll {
		if err := nestedUnarchive(ctx, dst); err != nil {
			return err
		}
	}
	return nil
}

// selectArchiver returns an archives.Archiver matching the longest suffix from dest.
func selectArchiver(dest string) archives.Archiver {
	var archiveExt string
	for ext := range archiverMap() {
		if strings.HasSuffix(dest, ext) && len(ext) > len(archiveExt) {
			archiveExt = ext
		}
	}
	return archiverMap()[archiveExt]
}

// archiverMap defines supported extensions to their Archiver implementations.
func archiverMap() map[string]archives.Archiver {
	// define common tar+compress combos once
	tar := func(c archives.Compression) archives.Archiver {
		return archives.CompressedArchive{Compression: c, Archival: archives.Tar{}}
	}
	return map[string]archives.Archiver{
		extensionZip:  archives.Zip{},
		extensionTar:  archives.Tar{},
		extensionTgz:  tar(archives.Gz{}),
		extensionGz:   tar(archives.Gz{}),
		extensionTbz:  tar(archives.Bz2{}),
		extensionBz2:  tar(archives.Bz2{}),
		extensionTbz2: tar(archives.Bz2{}),
		extensionTxz:  tar(archives.Xz{}),
		extensionXz:   tar(archives.Xz{}),
		extensionTzst: tar(archives.Zstd{}),
		extensionZst:  tar(archives.Zstd{}),
		extensionTbr:  tar(archives.Brotli{}),
		extensionBr:   tar(archives.Brotli{}),
		extensionTlz4: tar(archives.Lz4{}),
		extensionLz4:  tar(archives.Lz4{}),
		extensionTmz:  tar(archives.MinLZ{}),
		extensionMz:   tar(archives.MinLZ{}),
		extensionLzip: tar(archives.Lzip{}),
	}
}

// extract opens src, identifies format, and runs handler for each entry.
func extract(ctx context.Context, src, dst string, handler archives.FileHandler) (err error) {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()

	format, input, err := archives.Identify(ctx, src, file)
	if err != nil {
		return err
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("unsupported format for extraction: %T", format)
	}
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return err
	}
	return extractor.Extract(ctx, input, handler)
}

// basicHandler writes each file/dir/link to dst without modifications.
func basicHandler(dst string) archives.FileHandler {
	return func(_ context.Context, f archives.FileInfo) error {
		target := filepath.Join(dst, f.NameInArchive)
		switch {
		case f.IsDir():
			return os.MkdirAll(target, f.Mode())
		case f.LinkTarget != "":
			return os.Symlink(filepath.Join(dst, f.LinkTarget), target)
		default:
			return writeFile(target, f, os.O_CREATE|os.O_WRONLY)
		}
	}
}

// stripHandler drops leading paths and respects overwrite flag.
func stripHandler(dst string, strip int, overwrite bool) archives.FileHandler {
	flags := os.O_CREATE | os.O_WRONLY
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	return func(_ context.Context, fi archives.FileInfo) error {
		parts := strings.Split(fi.NameInArchive, "/")
		if len(parts) <= strip {
			return nil
		}
		rel := filepath.Join(parts[strip:]...)
		target := filepath.Join(dst, rel)
		switch {
		case fi.IsDir():
			return os.MkdirAll(target, fi.Mode())
		case fi.LinkTarget != "":
			return os.Symlink(fi.LinkTarget, target)
		default:
			return writeFile(target, fi, flags)
		}
	}
}

// extractFiltered handles filtered extraction of specified internal paths.
func extractFiltered(ctx context.Context, src, dst string, want []string, skipValidation bool) (err error) {
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open archive %q: %w", src, err)
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()

	format, input, err := archives.Identify(ctx, src, file)
	if err != nil {
		return fmt.Errorf("unable to identify archive %q: %w", src, err)
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("unsupported format for extraction: %T", format)
	}
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("unable to create destination %q: %w", dst, err)
	}

	found := make(map[string]bool, len(want))
	wantSet := make(map[string]bool, len(want))
	for _, name := range want {
		wantSet[name] = true
	}

	handler := func(_ context.Context, f archives.FileInfo) error {
		if !wantSet[f.NameInArchive] {
			return nil
		}
		found[f.NameInArchive] = true

		target := filepath.Join(dst, f.NameInArchive)
		switch {
		case f.IsDir():
			return os.MkdirAll(target, f.Mode())
		case f.LinkTarget != "":
			return os.Symlink(filepath.Join(dst, f.LinkTarget), target)
		default:
			return writeFile(target, f, os.O_CREATE|os.O_WRONLY)
		}
	}

	if err := extractor.Extract(ctx, input, handler); err != nil {
		return fmt.Errorf("error extracting filtered entries from %q: %w", src, err)
	}

	if !skipValidation {
		for _, name := range want {
			if !found[name] {
				return fmt.Errorf("file %q not found in archive %q", name, src)
			}
		}
	}
	return nil
}

// writeFile encapsulates file writing and copying.
func writeFile(target string, fi archives.FileInfo, flags int) (err error) {
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return err
	}
	out, err := os.OpenFile(target, flags, fi.Mode())
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()

	in, err := fi.Open()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, in.Close())
	}()

	_, err = io.Copy(out, in)
	return err
}

// nestedUnarchive takes a destination path and walks each file in the directory, unarchiving each.
func nestedUnarchive(ctx context.Context, dst string) error {
	err := filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, extensionTar) {
			dst := filepath.Join(strings.TrimSuffix(path, extensionTar), "..")
			// Unpack sboms.tar differently since it has a different folder structure than components
			if info.Name() == sbomFileName {
				dst = strings.TrimSuffix(path, extensionTar)
			}
			err := extract(ctx, path, dst, basicHandler(dst))
			if err != nil {
				return fmt.Errorf(lang.ErrUnarchive, path, err.Error())
			}
			err = os.Remove(path)
			if err != nil {
				return fmt.Errorf(lang.ErrRemoveFile, path, err.Error())
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to unarchive all nested tarballs: %w", err)
	}
	return nil
}
