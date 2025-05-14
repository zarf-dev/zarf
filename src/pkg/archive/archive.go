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

// Compress takes any number of source files and archives them into a compressed archive at dest path.
func Compress(ctx context.Context, sources []string, dest string, _ CompressOpts) error {
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dest, err)
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()

	mapping := make(map[string]string, len(sources))
	for _, src := range sources {
		mapping[src] = filepath.Base(src)
	}
	files, err := archives.FilesFromDisk(ctx, nil, mapping)
	if err != nil {
		return fmt.Errorf("failed to stat sources: %w", err)
	}

	archivers := map[string]archives.Archiver{
		extensionZip:  archives.Zip{},
		extensionTar:  archives.Tar{},
		extensionGz:   archives.CompressedArchive{Compression: archives.Gz{}, Archival: archives.Tar{}},
		extensionTgz:  archives.CompressedArchive{Compression: archives.Gz{}, Archival: archives.Tar{}},
		extensionBz2:  archives.CompressedArchive{Compression: archives.Bz2{}, Archival: archives.Tar{}},
		extensionTbz2: archives.CompressedArchive{Compression: archives.Bz2{}, Archival: archives.Tar{}},
		extensionTbz:  archives.CompressedArchive{Compression: archives.Bz2{}, Archival: archives.Tar{}},
		extensionXz:   archives.CompressedArchive{Compression: archives.Xz{}, Archival: archives.Tar{}},
		extensionTxz:  archives.CompressedArchive{Compression: archives.Xz{}, Archival: archives.Tar{}},
		extensionZst:  archives.CompressedArchive{Compression: archives.Zstd{}, Archival: archives.Tar{}},
		extensionTzst: archives.CompressedArchive{Compression: archives.Zstd{}, Archival: archives.Tar{}},
		extensionBr:   archives.CompressedArchive{Compression: archives.Brotli{}, Archival: archives.Tar{}},
		extensionTbr:  archives.CompressedArchive{Compression: archives.Brotli{}, Archival: archives.Tar{}},
		extensionLz4:  archives.CompressedArchive{Compression: archives.Lz4{}, Archival: archives.Tar{}},
		extensionTlz4: archives.CompressedArchive{Compression: archives.Lz4{}, Archival: archives.Tar{}},
		extensionLzip: archives.CompressedArchive{Compression: archives.Lzip{}, Archival: archives.Tar{}},
		extensionMz:   archives.CompressedArchive{Compression: archives.MinLZ{}, Archival: archives.Tar{}},
		extensionTmz:  archives.CompressedArchive{Compression: archives.MinLZ{}, Archival: archives.Tar{}},
	}

	// Find the longest matching extension
	var archiveExt string
	for ext := range archivers {
		if strings.HasSuffix(dest, ext) && len(ext) > len(archiveExt) {
			archiveExt = ext
		}
	}

	archiver, ok := archivers[archiveExt]
	if !ok {
		return fmt.Errorf("unsupported archive extension for %q", dest)
	}

	if err := archiver.Archive(ctx, out, files); err != nil {
		return fmt.Errorf("archive failed for %q: %w", dest, err)
	}

	return nil
}

// DecompressOpts provides optional parameters for Decompress
type DecompressOpts struct {
	// UnarchiveAll walks root of the archive and unpacks nested .tar files.
	UnarchiveAll bool

	// Files, if non-empty, means "only extract these exact archive-paths."
	Files []string

	// StripComponents drops this many leading path elements from every entry.
	StripComponents int

	// OverwriteExisting, if true, will truncate existing files instead of failing.
	OverwriteExisting bool

	// SkipValidation, if true, will skip the validation of a file being present in the archive.
	// This is used with unarchiveFiltered to avoid checking for files that are not in the archive.
	// This was a previous behavior that the new logic does not support.
	SkipValidation bool
}

// Decompress takes a Zarf package or arbitrary archive and decompresses it to dst.
func Decompress(ctx context.Context, sourceArchive, dst string, opts DecompressOpts) error {
	if len(opts.Files) > 0 {
		if err := unarchiveFiltered(ctx, sourceArchive, dst, opts.Files, opts.SkipValidation); err != nil {
			return fmt.Errorf("unable to decompress selected files: %w", err)
		}
		return nil
	}
	var err error
	if opts.StripComponents > 0 || opts.OverwriteExisting {
		err = unarchiveWithStrip(ctx, sourceArchive, dst,
			opts.StripComponents, opts.OverwriteExisting)
	} else {
		err = unarchive(ctx, sourceArchive, dst)
	}
	if err != nil {
		return fmt.Errorf("unable to perform decompression: %w", err)
	}
	// 3) nested unarchive step remains unchanged
	if opts.UnarchiveAll {
		if err := nestedUnarchive(ctx, dst); err != nil {
			return err
		}
	}
	return nil
}

// unarchiveWithStrip unpacks any supported archive, stripping `strip` path elements
// and opening files with or without truncate based on `overwrite`.
func unarchiveWithStrip(ctx context.Context, archivePath, dst string, strip int, overwrite bool) error {
	// open archive
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening %q: %w", archivePath, err)
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	// identify format (tar, tar.zst, zip, etc.)
	format, input, err := archives.Identify(ctx, filepath.Base(archivePath), f)
	if err != nil {
		return fmt.Errorf("identifying archive %q: %w", archivePath, err)
	}
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("format %T cannot extract", format)
	}

	// ensure dst exists
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("creating dest %q: %w", dst, err)
	}

	// choose flags for file creation
	flags := os.O_CREATE | os.O_WRONLY
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	handler := func(_ context.Context, fi archives.FileInfo) error {
		parts := strings.Split(fi.NameInArchive, "/")
		if len(parts) <= strip {
			// nothing left after stripping → skip
			return nil
		}
		rel := filepath.Join(parts[strip:]...)
		target := filepath.Join(dst, rel)

		switch {
		case fi.IsDir():
			return os.MkdirAll(target, fi.Mode())

		case fi.LinkTarget != "":
			// recreate symlink (we do not strip link targets here)
			return os.Symlink(fi.LinkTarget, target)

		default:
			// regular file
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

			if _, err := io.Copy(out, in); err != nil {
				return err
			}
			return nil
		}
	}

	if err := extractor.Extract(ctx, input, handler); err != nil {
		return fmt.Errorf("extracting %q: %w", archivePath, err)
	}
	return nil
}

// unarchiveFiltered extracts only the given list of archive‐internal filenames
// into dst, and errors if any one of them was not found.
func unarchiveFiltered(ctx context.Context, src, dst string, want []string, skipValidation bool) error {
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open archive %q: %w", src, err)
	}
	defer func() {
		cErr := file.Close()
		err = errors.Join(err, cErr)
	}()

	format, input, err := archives.Identify(ctx, src, file)
	if err != nil {
		return fmt.Errorf("unable to identify archive %q: %w", src, err)
	}

	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("unsupported format for extraction: %T", format)
	}

	// We'll track which ones we actually saw
	found := make(map[string]bool, len(want))
	wantSet := make(map[string]bool, len(want))
	for _, name := range want {
		wantSet[name] = true
	}

	// Ensure dst exists
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("unable to create destination %q: %w", dst, err)
	}

	handler := func(_ context.Context, f archives.FileInfo) error {
		// skip anything not in our list
		if !wantSet[f.NameInArchive] {
			return nil
		}
		found[f.NameInArchive] = true

		target := filepath.Join(dst, f.NameInArchive)

		switch {
		case f.IsDir():
			return os.MkdirAll(target, f.Mode())

		case f.LinkTarget != "":
			linkDest := filepath.Join(dst, f.LinkTarget)
			return os.Symlink(linkDest, target)

		default:
			if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				cErr := out.Close()
				err = errors.Join(err, cErr)
			}()

			in, err := f.Open()
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, in.Close())
			}()

			_, err = io.Copy(out, in)
			return err
		}
	}

	if err := extractor.Extract(ctx, input, handler); err != nil {
		return fmt.Errorf("error extracting filtered entries from %q: %w", src, err)
	}

	// verify we got them all
	for _, name := range want {
		if !found[name] && !skipValidation {
			return fmt.Errorf("file %q not found in archive %q", name, src)
		}
	}
	return nil
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
			err := unarchive(ctx, path, dst)
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

// unarchive opens src, identifies its format, and extracts into dst.
func unarchive(ctx context.Context, src, dst string) error {
	// Open the archive file
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open archive %q: %w", src, err)
	}
	defer func() {
		err = errors.Join(err, file.Close())
	}()

	// Identify format & get an input stream
	format, input, err := archives.Identify(ctx, src, file)
	if err != nil {
		return fmt.Errorf("unable to identify archive %q: %w", src, err)
	}

	// Assert that it supports extraction
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("unsupported format for extraction: %T", format)
	}

	// Ensure dst exists
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("unable to create destination %q: %w", dst, err)
	}

	// Define how each entry is written to disk
	handler := func(_ context.Context, f archives.FileInfo) error {
		target := filepath.Join(dst, f.NameInArchive)

		switch {
		case f.IsDir():
			// directory
			return os.MkdirAll(target, f.Mode())

		case f.LinkTarget != "":
			// symlink
			linkDest := filepath.Join(dst, f.LinkTarget)
			return os.Symlink(linkDest, target)

		default:
			// regular file
			if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, out.Close())
			}()

			in, err := f.Open()
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, in.Close())
			}()

			_, err = io.Copy(out, in)
			return err
		}
	}

	// Perform extraction
	if err := extractor.Extract(ctx, input, handler); err != nil {
		return fmt.Errorf("unable to extract %q: %w", src, err)
	}
	return nil
}
