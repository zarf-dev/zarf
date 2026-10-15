// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package componentarchive writes reproducible component archives and reads both archive layouts.
package componentarchive

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/pkg/archive"
)

// Create archives a directory with stable paths, ordering, timestamps, and ownership.
// An empty prefix produces a name-independent archive; a component name produces the legacy layout.
func Create(ctx context.Context, dirPath, dirPrefix, tarballPath string, overrideMode bool) (err error) {
	tb, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("error creating tarball: %w", err)
	}
	defer func() {
		err = errors.Join(err, tb.Close())
	}()

	tw := tar.NewWriter(tb)
	defer func() {
		err = errors.Join(err, tw.Close())
	}()

	// Walk through the directory and process each file
	return filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		link := ""
		if info.Mode().Type() == os.ModeSymlink {
			link, err = os.Readlink(filePath)
			if err != nil {
				return fmt.Errorf("error reading symlink: %w", err)
			}
		}

		// Create a new header
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return fmt.Errorf("error creating tar header: %w", err)
		}

		// Strip non-deterministic header data
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""

		// When run on windows the header mode will set all permission octals to the same value as the first octal.
		// A file created with 0o700 will return 0o777 when read back. This discrepancy causes differences between packages
		// created on Windows and Linux.
		// https://medium.com/@MichalPristas/go-and-file-perms-on-windows-3c944d55dd44
		// To mitigate this difference we zero all but the last permission octal when writing files to the tar. Making sure
		// that when unpackaged files from packages created on Windows and Linux will have the same permissions.
		// The &^ operator called AND NOT sets the bits to 0 in the left hand if the right hand bits are 1.
		// https://medium.com/learning-the-go-programming-language/bit-hacking-with-go-e0acee258827
		if overrideMode {
			header.Mode = header.Mode &^ 0o077
		}

		// Ensure the header's name is correctly set relative to the base directory
		name, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}
		name = filepath.Join(dirPrefix, name)
		name = filepath.ToSlash(name)
		header.Name = name

		// Write the header to the tarball
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("error writing header: %w", err)
		}

		// If it's a file, write its content
		if info.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("error opening file: %w", err)
			}
			_, copyErr := io.Copy(tw, file)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return fmt.Errorf("error writing file to tarball: %w", err)
			}
		}

		return nil
	})
}

// Extract accepts name-independent archives and legacy archives prefixed with the component name.
// It returns the directory containing the component's resource directories.
func Extract(ctx context.Context, source, destination, componentName string) (string, error) {
	file, err := os.Open(source)
	if err != nil {
		return "", err
	}
	header, readErr := tar.NewReader(file).Next()
	if err := errors.Join(readErr, file.Close()); err != nil {
		return "", err
	}
	root := destination
	if path.Clean(header.Name) != "." {
		if strings.SplitN(path.Clean(header.Name), "/", 2)[0] != componentName {
			return "", fmt.Errorf("component %s archive has an unexpected root %q", componentName, header.Name)
		}
		root = filepath.Join(destination, componentName)
	}
	if err := archive.Decompress(ctx, source, destination, archive.DecompressOpts{}); err != nil {
		return "", err
	}
	return root, nil
}
