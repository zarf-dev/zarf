// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package archive contains the SDK for Zarf archival and compression.
package archive

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
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
	// sbomFileName is the filename for SBOM archives in a Zarf package.
	sbomFileName = "sboms.tar"
	// documentationFileName is the filename for Documentation archives in a Zarf package
	documentationFileName = "documentation.tar"
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
func unarchive(ctx context.Context, extractor archives.Extractor, src, dst string) (err error) {
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("creating dest %q: %w", dst, err)
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return fmt.Errorf("opening root %q: %w", dst, err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	return withArchive(src, extractor, func(ex archives.Extractor, input io.Reader) error {
		if err := ex.Extract(ctx, input, defaultHandler(root)); err != nil {
			return fmt.Errorf("extracting %q: %w", src, err)
		}
		return nil
	})
}

// unarchiveWithStrip extracts all entries from src into dst, stripping the
// first 'strip' path components and optionally overwriting existing files.
func unarchiveWithStrip(ctx context.Context, extractor archives.Extractor, src, dst string, strip int, overwrite bool) (err error) {
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("creating dest %q: %w", dst, err)
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return fmt.Errorf("opening root %q: %w", dst, err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	return withArchive(src, extractor, func(ex archives.Extractor, input io.Reader) error {
		if err := ex.Extract(ctx, input, stripHandler(root, strip, overwrite)); err != nil {
			return fmt.Errorf("extracting %q with strip: %w", src, err)
		}
		return nil
	})
}

// unarchiveFiltered extracts only the specified 'want' entries from src into dst.
// It records found entries and, unless skipValidation is true, returns an error
// if any requested entry is missing.
func unarchiveFiltered(ctx context.Context, extractor archives.Extractor, src, dst string, want []string, skipValidation bool) (err error) {
	found := make(map[string]bool, len(want))
	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
	}
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return fmt.Errorf("creating dest %q: %w", dst, err)
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return fmt.Errorf("opening root %q: %w", dst, err)
	}
	defer func() { err = errors.Join(err, root.Close()) }()

	err = withArchive(src, extractor, func(ex archives.Extractor, input io.Reader) error {
		handler := filterHandler(root, wantSet, found)
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
// It uses WalkDir so that symlinks are visible via d.Type() and skipped,
// preventing the walk from following symlinks into directories.
func nestedUnarchive(ctx context.Context, extractor archives.Extractor, dst string) error {
	return filepath.WalkDir(dst, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(path, extensionTar) {
			return nil
		}
		outDir := filepath.Join(strings.TrimSuffix(path, extensionTar), "..")
		if d.Name() == sbomFileName || d.Name() == documentationFileName {
			outDir = strings.TrimSuffix(path, extensionTar)
		}
		if err := unarchive(ctx, extractor, path, outDir); err != nil {
			return fmt.Errorf(lang.ErrUnarchive, path, err)
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf(lang.ErrRemoveFile, path, err)
		}
		return nil
	})
}

// defaultHandler returns an archive.Entry handler that writes each entry within root.
func defaultHandler(root *os.Root) func(_ context.Context, f archives.FileInfo) error {
	return func(_ context.Context, f archives.FileInfo) error {
		return writeEntry(root, f.NameInArchive, f.LinkTarget, f, os.O_CREATE|os.O_WRONLY)
	}
}

// stripHandler returns an archive.Entry handler that writes each entry
// within root, stripping the first 'strip' path components.
func stripHandler(root *os.Root, strip int, overwrite bool) func(_ context.Context, f archives.FileInfo) error {
	return func(_ context.Context, f archives.FileInfo) error {
		parts := strings.Split(f.NameInArchive, "/")
		if len(parts) <= strip {
			return nil
		}
		rel := path.Join(parts[strip:]...)
		if rel == "" || rel == "." {
			return nil
		}
		// Hardlink targets in tar are archive-root-relative, so the
		// same prefix stripping must be applied to the target.
		linkTarget := f.LinkTarget
		if linkTarget != "" {
			if hdr, ok := f.Header.(*tar.Header); ok && hdr.Typeflag == tar.TypeLink {
				targetParts := strings.Split(linkTarget, "/")
				if len(targetParts) > strip {
					linkTarget = path.Join(targetParts[strip:]...)
				}
			}
		}
		flags := os.O_CREATE | os.O_WRONLY
		if overwrite {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_EXCL
		}
		return writeEntry(root, rel, linkTarget, f, flags)
	}
}

// filterHandler returns an archive.Entry handler that writes only entries
// whose names are in the 'wantSet'. It records found entries in 'found'.
func filterHandler(root *os.Root, wantSet, found map[string]bool) func(_ context.Context, f archives.FileInfo) error {
	return func(_ context.Context, f archives.FileInfo) error {
		if !wantSet[f.NameInArchive] {
			return nil
		}
		found[f.NameInArchive] = true
		return writeEntry(root, f.NameInArchive, f.LinkTarget, f, os.O_CREATE|os.O_WRONLY)
	}
}

// writeEntry validates and dispatches an archive entry within root.
// Directory and file operations use os.Root methods, which provide
// kernel-enforced path traversal and symlink escape protection.
//
// The linkTarget parameter is passed separately from f.LinkTarget so
// callers like stripHandler can adjust it (e.g., strip a path prefix
// from hardlink targets, which are archive-root-relative in tar).
//
// For links, archives.FileInfo.LinkTarget is populated for both symlinks
// (tar.TypeSymlink) and hardlinks (tar.TypeLink). They require different
// handling:
//   - Hardlinks: targets are relative to the archive root. os.Root.Link
//     validates that both paths stay within root.
//   - Symlinks: targets are relative to the link's parent directory.
//     os.Root.Symlink does not validate the target, so validateSymlink
//     checks that the target resolves within root.
//
// For non-tar archives (zip), the type assertion to *tar.Header fails and
// the entry falls through to the symlink path, which is correct since zip
// has no hardlink concept.
func writeEntry(root *os.Root, rel, linkTarget string, f archives.FileInfo, flags int) error {
	if err := validateEntryName(rel); err != nil {
		return err
	}
	switch {
	case f.IsDir():
		return root.MkdirAll(rel, f.Mode().Perm())
	case linkTarget != "":
		if err := validateEntryName(linkTarget); err != nil {
			return err
		}
		if hdr, ok := f.Header.(*tar.Header); ok && hdr.Typeflag == tar.TypeLink {
			return root.Link(linkTarget, rel)
		}
		if err := validateSymlink(rel, linkTarget); err != nil {
			return err
		}
		// Since we're now operating on the filesystem, we need the actual path
		return root.Symlink(filepath.FromSlash(linkTarget), rel)
	default:
		return writeFile(root, rel, f, flags)
	}
}

// windowsReservedNames contains device names that Windows resolves to device
// drivers regardless of directory. The object manager intercepts these before
// path resolution, so os.Root cannot prevent access to them.
var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM0": true, "COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT0": true, "LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// validateEntryName rejects archive entry names and symlink targets that
// contain constructs unsafe on Windows. These checks run on all platforms
// because archives may be created on one OS and extracted on another.
func validateEntryName(name string) error {
	if name == "" {
		return fmt.Errorf("empty entry name")
	}
	// Reject backslashes — they are not valid in POSIX tar entry names and
	// act as path separators on Windows, which can cause strip logic and
	// other path operations to produce incorrect results.
	if strings.ContainsRune(name, '\\') {
		return fmt.Errorf("entry name %q contains backslash", name)
	}
	for part := range strings.SplitSeq(name, "/") {
		// Reject colons — covers drive letters (C:file), drive-relative
		// paths (C:relative), and NTFS alternate data streams (file:stream).
		if strings.ContainsRune(part, ':') {
			return fmt.Errorf("path component %q contains invalid character ':'", part)
		}
		// Reject trailing dots and spaces — Windows/NTFS silently strips
		// these, causing "file.txt." to write to "file.txt" which can
		// bypass filter matching. Skip "." and ".." which are valid
		// path components handled by other validation.
		if part != "." && part != ".." && part != strings.TrimRight(part, ". ") {
			return fmt.Errorf("path component %q has trailing dots or spaces", part)
		}
		// Reject Windows reserved device names. These resolve to kernel
		// device drivers regardless of directory path, even under os.Root.
		// Windows also treats names with extensions as reserved: NUL.txt
		// and NUL.tar.gz both resolve to the NUL device.
		upper := strings.ToUpper(part)
		if windowsReservedNames[upper] {
			return fmt.Errorf("path component %q is a reserved device name", part)
		}
		if dot := strings.IndexByte(upper, '.'); dot > 0 && windowsReservedNames[upper[:dot]] {
			return fmt.Errorf("path component %q uses a reserved device name with extension", part)
		}
	}
	return nil
}

// validateSymlink checks that a symlink target is relative and resolves within
// the root directory. It resolves the target from the symlink's parent directory
// (matching filesystem symlink semantics) and rejects targets that escape.
func validateSymlink(rel, linkTarget string) error {
	if linkTarget == "" {
		return fmt.Errorf("empty symlink target for %q", rel)
	}
	// Reject rooted paths first — both /foo and \foo. On Windows,
	// filepath.IsAbs returns false for /foo (rooted, drive-relative),
	// so we must check the leading character before calling IsAbs.
	if linkTarget[0] == '/' || linkTarget[0] == '\\' {
		return fmt.Errorf("symlink target %q is absolute or rooted, which is not allowed", linkTarget)
	}
	if filepath.IsAbs(linkTarget) {
		return fmt.Errorf("absolute symlink target %q is not allowed", linkTarget)
	}
	// Reject drive-letter and UNC paths that filepath.IsAbs may not catch
	// on all platforms (e.g., C:relative is not "absolute" but carries a
	// volume prefix that redirects resolution).
	if filepath.VolumeName(linkTarget) != "" {
		return fmt.Errorf("symlink target %q contains a volume name", linkTarget)
	}
	// Use path (POSIX, forward-slash only) for all archive path arithmetic.
	// Archive entry names are always POSIX-style; filepath would normalize
	// separators to backslash on Windows, breaking the escape check.
	resolved := path.Clean(path.Join(path.Dir(rel), linkTarget))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return fmt.Errorf("symlink target %q escapes root directory", linkTarget)
	}
	return nil
}

// writeFile creates parent directories and writes file contents within root.
func writeFile(root *os.Root, rel string, fi archives.FileInfo, flags int) (err error) {
	if dir := path.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, dirPerm); err != nil {
			return err
		}
	}
	out, err := root.OpenFile(rel, flags, fi.Mode().Perm())
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
