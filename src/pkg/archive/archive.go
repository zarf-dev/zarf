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

	"github.com/mholt/archiver/v3"
	"github.com/mholt/archives"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/layout"
)

const rwxPerm = 0o755

// CompressOpts is a placeholder for future optional Compress params
type CompressOpts struct{}

// Compress takes any number of source files and archives them into a tarball at dest path.
// TODO(mkcp): Migrate to mholt/archives, see CVE-2024-0406
func Compress(_ context.Context, sources []string, dest string, _ CompressOpts) error {
	return archiver.Archive(sources, dest)
}

// DecompressOpts provides optional parameters for Decompress
type DecompressOpts struct {
	// UnarchiveAll, when enabled, walks the sourceArchive and unarchives everything at the root of the archive.
	// NOTE(mkcp): This is equivalent to a recursive walk with depth 1.
	UnarchiveAll bool
}

// Decompress takes Zarf package or arbitrary archive and decompresses it to the path at dest with options.
func Decompress(ctx context.Context, sourceArchive, dst string, opts DecompressOpts) error {
	if ctx == nil {
		ctx = context.Background()
	}
	err := unarchive(ctx, sourceArchive, dst)
	if err != nil {
		return fmt.Errorf("unable to perform decompression: %w", err)
	}
	if opts.UnarchiveAll {
		err = nestedUnarchive(ctx, dst)
		if err != nil {
			return err
		}
	}
	return nil
}

// nestedWalk takes a destination path and walks each file in the directory, unarchiving each.
func nestedUnarchive(ctx context.Context, dst string) error {
	err := filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".tar") {
			dst := filepath.Join(strings.TrimSuffix(path, ".tar"), "..")
			// Unpack sboms.tar differently since it has a different folder structure than components
			if info.Name() == layout.SBOMTar {
				dst = strings.TrimSuffix(path, ".tar")
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
	if err := os.MkdirAll(dst, rwxPerm); err != nil {
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
			if err := os.MkdirAll(filepath.Dir(target), rwxPerm); err != nil {
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
