// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package split splits and re-assembles files
package split

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// SplitFileMetadata contains info about a split file.
type SplitFileMetadata struct {
	// The sha256sum of the file
	Sha256Sum string
	// The size of the file in bytes
	Bytes int64
	// The number of parts the file is split into
	Count int
}

// SplitFile splits a file into several parts and returns the path to part000
// part000 always holds the splitFileMetadata. The remaining parts hold a chunkSize number of bytes of the original file.
func SplitFile(ctx context.Context, srcPath string, chunkSize int) (_ string, err error) {
	// Remove any existing split files
	existingChunks, err := filepath.Glob(srcPath + ".part*")
	if err != nil {
		return "", err
	}
	for _, chunk := range existingChunks {
		err := os.Remove(chunk)
		if err != nil {
			return "", err
		}
	}
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	// Ensure we close our sourcefile, even if we error out.
	defer func() {
		err2 := srcFile.Close()
		// Ignore if file is already closed
		if !errors.Is(err2, os.ErrClosed) {
			err = errors.Join(err, err2)
		}
	}()

	fi, err := srcFile.Stat()
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	fileCount := 0
	// TODO(mkcp): The inside of this loop should be wrapped in a closure so we can close the destination file each
	//   iteration as soon as we're done writing.
	for {
		path := fmt.Sprintf("%s.part%03d", srcPath, fileCount+1)
		dstFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if err != nil {
			return "", err
		}
		defer func(dstFile *os.File) {
			err2 := dstFile.Close()
			// Ignore if file is already closed
			if !errors.Is(err2, os.ErrClosed) {
				err = errors.Join(err, err2)
			}
		}(dstFile)

		written, copyErr := io.CopyN(dstFile, srcFile, int64(chunkSize))
		if copyErr != nil && !errors.Is(copyErr, io.EOF) {
			return "", err
		}

		_, err = dstFile.Seek(0, io.SeekStart)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(hash, dstFile)
		if err != nil {
			return "", err
		}

		// EOF error could be returned on 0 bytes written.
		if written == 0 {
			// NOTE(mkcp): We have to close the file before removing it or windows will break with a file-in-use err.
			err = dstFile.Close()
			if err != nil {
				return "", err
			}
			err = os.Remove(path)
			if err != nil {
				return "", err
			}
			break
		}

		fileCount++
		if errors.Is(copyErr, io.EOF) {
			break
		}
	}

	// Remove original file
	// NOTE(mkcp): We have to close the file before removing or windows can break with a file-in-use err.
	err = srcFile.Close()
	if err != nil {
		return "", err
	}
	err = os.Remove(srcPath)
	if err != nil {
		return "", err
	}

	// Write header file
	data := SplitFileMetadata{
		Count:     fileCount,
		Bytes:     fi.Size(),
		Sha256Sum: fmt.Sprintf("%x", hash.Sum(nil)),
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("unable to marshal the split package data: %w", err)
	}
	path := fmt.Sprintf("%s.part000", srcPath)
	if err := os.WriteFile(path, b, 0644); err != nil {
		return "", fmt.Errorf("unable to write the file %s: %w", path, err)
	}
	logger.From(ctx).Info("package split across files", "count", fileCount+1)
	return path, nil
}

// ReassembleFile takes a directory containing split files, reassembles those files into the destination, then the split files.
func ReassembleFile(src, dest string) (err error) {
	pattern := strings.Replace(src, ".part000", ".part*", 1)
	splitFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to find split tarball files: %w", err)
	}
	if len(splitFiles) == 0 {
		return fmt.Errorf("no split files with pattern %s found", pattern)
	}
	slices.Sort(splitFiles)

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()

	for i, part := range splitFiles {
		if i == 0 {
			// validate metadata
			data, err := os.ReadFile(part)
			if err != nil {
				return err
			}
			var meta SplitFileMetadata
			err = json.Unmarshal(data, &meta)
			if err != nil {
				return err
			}
			expected := len(splitFiles) - 1
			if meta.Count != expected {
				return fmt.Errorf("split parts mismatch: expected %d, got %d", expected, meta.Count)
			}
			continue
		}

		// Create a new scope for the file so the defer close happens during each loop rather than once the function completes
		err := func() (err error) {
			f, err := os.Open(part)
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, f.Close())
			}()

			_, err = io.Copy(out, f)
			return err
		}()
		if err != nil {
			return err
		}
	}

	for _, file := range splitFiles {
		err := os.Remove(file)
		if err != nil {
			return err
		}
	}

	return nil
}
