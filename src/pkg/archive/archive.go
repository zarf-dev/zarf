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
	"slices"
	"strings"

	"github.com/mholt/archives"
	"github.com/zarf-dev/zarf/src/config/lang"
)

const (
	// extensionTar is the standard TAR archive extension.
	extensionTar = ".tar"
	// extensionZip is the standard ZIP archive extension.
	extensionZip = ".zip"
	// extensionGz and extensionTgz denote gzip-compressed tarballs.
	extensionGz  = ".tar.gz"
	extensionTgz = ".tgz"
	// extensionBz2, extensionTbz2, and extensionTbz denote bzip2-compressed tarballs.
	extensionBz2  = ".tar.bz2"
	extensionTbz2 = ".tbz2"
	extensionTbz  = ".tbz"
	// extensionXz and extensionTxz denote xz-compressed tarballs.
	extensionXz  = ".tar.xz"
	extensionTxz = ".txz"
	// extensionZst and extensionTzst denote zstd-compressed tarballs.
	extensionZst  = ".tar.zst"
	extensionTzst = ".tzst"
	// extensionBr and extensionTbr denote brotli-compressed tarballs.
	extensionBr  = ".tar.br"
	extensionTbr = ".tbr"
	// extensionLz4 and extensionTlz4 denote lz4-compressed tarballs.
	extensionLz4  = ".tar.lz4"
	extensionTlz4 = ".tlz4"
	// extensionLzip denotes lzip-compressed tarballs.
	extensionLzip = ".tar.lz"
	// extensionMz and extensionTmz denote minLZ-compressed tarballs.
	extensionMz  = ".tar.mz"
	extensionTmz = ".tmz"
	// sbomFileName is the default filename for nested SBOM archives.
	sbomFileName = "sboms.tar"
	// dirPerm defines the permission bits for created directories.
	dirPerm = 0o755
	// filePerm defines the permission bits for created files (unused).
	filePerm = 0o644
)

// archivers maps file extensions to their corresponding Archiver implementation.
var archivers = initArchivers()

// initArchivers constructs the archivers map by grouping extensions by compression type.
func initArchivers() map[string]archives.Archival {
	m := map[string]archives.Archival{
		extensionTar: archives.Tar{},
		extensionZip: archives.Zip{},
	}
	// group extensions by the compression they use
	groups := []struct {
		exts []string
		comp archives.Compression
	}{
		{[]string{extensionGz, extensionTgz}, archives.Gz{}},
		{[]string{extensionBz2, extensionTbz2, extensionTbz}, archives.Bz2{}},
		{[]string{extensionXz, extensionTxz}, archives.Xz{}},
		{[]string{extensionZst, extensionTzst}, archives.Zstd{}},
		{[]string{extensionBr, extensionTbr}, archives.Brotli{}},
		{[]string{extensionLz4, extensionTlz4}, archives.Lz4{}},
		{[]string{extensionLzip}, archives.Lzip{}},
		{[]string{extensionMz, extensionTmz}, archives.MinLZ{}},
	}

	for _, g := range groups {
		for _, ext := range g.exts {
			m[ext] = archives.CompressedArchive{
				Compression: g.comp,
				Archival:    archives.Tar{},
				Extraction:  archives.Tar{},
			}
		}
	}
	return m
}

// findArchiver returns the best-matching Archiver for a filename based on its extension.
func findArchiver(name string) (archives.Archiver, error) {
	var best string
	for ext := range archivers {
		if strings.HasSuffix(name, ext) && len(ext) > len(best) {
			best = ext
		}
	}
	if a, ok := archivers[best]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("unsupported archive extension for %q", name)
}

// CompressOpts holds future optional parameters for Compress.
type CompressOpts struct{}

// Compress archives the given sources into dest, selecting the format by dest's extension.
func Compress(ctx context.Context, sources []string, dest string, _ CompressOpts) (err error) {
	if len(sources) == 0 {
		return fmt.Errorf("sources cannot be empty")
	}
	if dest == "" {
		return fmt.Errorf("dest cannot be empty")
	}

	// Ensure dest parent directories exist
	err = os.MkdirAll(filepath.Dir(dest), dirPerm)
	if err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", dest, err)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dest, err)
	}
	defer func() { err = errors.Join(err, out.Close()) }()

	// map local paths to archive names
	mapping := make(map[string]string, len(sources))
	for _, src := range sources {
		mapping[src] = filepath.Base(src)
	}
	files, err := archives.FilesFromDisk(ctx, &archives.FromDiskOptions{
		ClearAttributes: true,
	}, mapping)
	if err != nil {
		return fmt.Errorf("failed to stat sources: %w", err)
	}

	// Sort files by NameInArchive to ensure deterministic tar creation
	// FilesFromDisk iterates over a map which has non-deterministic ordering
	slices.SortFunc(files, func(a, b archives.FileInfo) int {
		if a.NameInArchive < b.NameInArchive {
			return -1
		}
		if a.NameInArchive > b.NameInArchive {
			return 1
		}
		return 0
	})

	archiver, err := findArchiver(dest)
	if err != nil {
		return err
	}
	if err := archiver.Archive(ctx, out, files); err != nil {
		return fmt.Errorf("archive failed for %q: %w", dest, err)
	}
	return nil
}

// DecompressOpts defines optional behavior for Decompress.
type DecompressOpts struct {
	// UnarchiveAll enables recursive unpacking of nested .tar files.
	UnarchiveAll bool
	// Files restricts extraction to these archive paths if non-empty.
	Files []string
	// StripComponents drops leading path elements from each entry.
	StripComponents int
	// OverwriteExisting truncates existing files instead of erroring.
	OverwriteExisting bool
	// SkipValidation suppresses errors for missing Files entries.
	SkipValidation bool
	// Extractor allows the user to specify which extractor should be used for decompression.
	// If this is not set it will be determined automatically from the file extension
	Extractor archives.Extractor
}

// Decompress extracts source into dst, using strip or filter logic per opts, then optionally nests.
func Decompress(ctx context.Context, source, dst string, opts DecompressOpts) error {
	switch {
	case len(opts.Files) > 0:
		return unarchiveFiltered(ctx, opts.Extractor, source, dst, opts.Files, opts.SkipValidation)
	case opts.StripComponents > 0 || opts.OverwriteExisting:
		if err := unarchiveWithStrip(ctx, opts.Extractor, source, dst, opts.StripComponents, opts.OverwriteExisting); err != nil {
			return fmt.Errorf("unable to decompress: %w", err)
		}
	default:
		if err := unarchive(ctx, opts.Extractor, source, dst); err != nil {
			return fmt.Errorf("unable to decompress: %w", err)
		}
	}

	if opts.UnarchiveAll {
		if err := nestedUnarchive(ctx, opts.Extractor, dst); err != nil {
			return err
		}
	}
	return nil
}

// withArchive opens, identifies, and creates and asserts an extractor if one is not given
func withArchive(path string, extractor archives.Extractor, fn func(ex archives.Extractor, input io.Reader) error) (err error) {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %q: %w", path, err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	if extractor == nil {
		format, err := findArchiver(path)
		if err != nil {
			return fmt.Errorf("identifying %q: %w", path, err)
		}
		ex, ok := format.(archives.Extractor)
		if !ok {
			return fmt.Errorf("format %T cannot extract", format)
		}
		extractor = ex
	}
	return fn(extractor, f)
}

// unarchive extracts all entries from src into dst using the defaultHandler.
// It ensures the destination directory exists before extraction.
func unarchive(ctx context.Context, extractor archives.Extractor, src, dst string) error {
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("creating dest %q: %w", dst, err)
	}
	return withArchive(src, extractor, func(ex archives.Extractor, input io.Reader) error {
		if err := ex.Extract(ctx, input, defaultHandler(dst)); err != nil {
			return fmt.Errorf("extracting %q: %w", src, err)
		}
		return nil
	})
}

// unarchiveWithStrip extracts all entries from src into dst, stripping the
// first 'strip' path components and optionally overwriting existing files.
func unarchiveWithStrip(ctx context.Context, extractor archives.Extractor, src, dst string, strip int, overwrite bool) error {
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("creating dest %q: %w", dst, err)
	}
	return withArchive(src, extractor, func(ex archives.Extractor, input io.Reader) error {
		if err := ex.Extract(ctx, input, stripHandler(dst, strip, overwrite)); err != nil {
			return fmt.Errorf("extracting %q with strip: %w", src, err)
		}
		return nil
	})
}

// unarchiveFiltered extracts only the specified 'want' entries from src into dst.
// It records found entries and, unless skipValidation is true, returns an error
// if any requested entry is missing.
func unarchiveFiltered(ctx context.Context, extractor archives.Extractor, src, dst string, want []string, skipValidation bool) error {
	found := make(map[string]bool, len(want))
	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
	}
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("creating dest %q: %w", dst, err)
	}

	err := withArchive(src, extractor, func(ex archives.Extractor, input io.Reader) error {
		handler := filterHandler(dst, wantSet, found)
		return ex.Extract(ctx, input, handler)
	})
	if err != nil {
		return fmt.Errorf("filtered extract of %q: %w", src, err)
	}

	if !skipValidation {
		for _, w := range want {
			if !found[w] {
				return fmt.Errorf("file %q not found in archive %q", w, src)
			}
		}
	}
	return nil
}

// nestedUnarchive walks dst and unarchives each .tar file it finds.
func nestedUnarchive(ctx context.Context, extractor archives.Extractor, dst string) error {
	return filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, extensionTar) {
			outDir := filepath.Join(strings.TrimSuffix(path, extensionTar), "..")
			if info.Name() == sbomFileName {
				outDir = strings.TrimSuffix(path, extensionTar)
			}
			if err := unarchive(ctx, extractor, path, outDir); err != nil {
				return fmt.Errorf(lang.ErrUnarchive, path, err.Error())
			}
			if err := os.Remove(path); err != nil {
				return fmt.Errorf(lang.ErrRemoveFile, path, err.Error())
			}
		}
		return nil
	})
}

// defaultHandler returns an archive.Entry handler that writes each entry under dst.
// It preserves directory structure, symlinks, and file contents.
func defaultHandler(dst string) func(_ context.Context, f archives.FileInfo) (err error) {
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

// stripHandler returns an archive.Entry handler that writes each entry under dst,
// stripping the first 'strip' path components.
func stripHandler(dst string, strip int, overwrite bool) func(_ context.Context, f archives.FileInfo) error {
	return func(_ context.Context, f archives.FileInfo) error {
		parts := strings.Split(f.NameInArchive, "/")
		if len(parts) <= strip {
			return nil
		}
		rel := filepath.Join(parts[strip:]...)
		target := filepath.Join(dst, rel)

		switch {
		case f.IsDir():
			return os.MkdirAll(target, f.Mode())
		case f.LinkTarget != "":
			return os.Symlink(f.LinkTarget, target)
		default:
			flags := os.O_CREATE | os.O_WRONLY
			if overwrite {
				flags |= os.O_TRUNC
			} else {
				flags |= os.O_EXCL
			}
			return writeFile(target, f, flags)
		}
	}
}

// filterHandler returns an archive.Entry handler that writes only entries
// whose names are in the 'wantSet'. It records found entries in 'found'.
// If an entry is not found in the archive, it returns an error unless 'skipValidation' is true.
func filterHandler(dst string, wantSet, found map[string]bool) func(_ context.Context, f archives.FileInfo) error {
	return func(_ context.Context, f archives.FileInfo) error {
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
