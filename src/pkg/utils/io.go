// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

const (
	tmpPathPrefix = "zarf-"
)

// MakeTempDir creates a temp directory with the zarf- prefix.
func MakeTempDir(basePath string) (string, error) {
	if basePath != "" {
		if err := helpers.CreateDirectory(basePath, helpers.ReadWriteExecuteUser); err != nil {
			return "", err
		}
	}

	tmp, err := os.MkdirTemp(basePath, tmpPathPrefix)
	if err != nil {
		return "", err
	}

	message.Debug("Using temporary directory:", tmp)

	return tmp, nil
}

// GetFinalExecutablePath returns the absolute path to the current executable, following any symlinks along the way.
func GetFinalExecutablePath() (string, error) {
	message.Debug("utils.GetExecutablePath()")

	binaryPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// In case the binary is symlinked somewhere else, get the final destination
	linkedPath, err := filepath.EvalSymlinks(binaryPath)
	return linkedPath, err
}

// GetFinalExecutableCommand returns the final path to the Zarf executable including and library prefixes and overrides.
func GetFinalExecutableCommand() (string, error) {
	// In case the binary is symlinked somewhere else, get the final destination
	zarfCommand, err := GetFinalExecutablePath()
	if err != nil {
		return zarfCommand, err
	}

	if config.ActionsCommandZarfPrefix != "" {
		zarfCommand = fmt.Sprintf("%s %s", zarfCommand, config.ActionsCommandZarfPrefix)
	}

	// If a library user has chosen to override config to use system Zarf instead, reset the binary path.
	if config.ActionsUseSystemZarf {
		zarfCommand = "zarf"
	}

	return zarfCommand, err
}

// SplitFile will take a srcFile path and split it into files based on chunkSizeBytes
// the first file will be a metadata file containing:
// - sha256sum of the original file
// - number of bytes in the original file
// - number of files the srcFile was split into
// SplitFile will delete the original file
//
// Returns:
// - fileNames: list of file paths srcFile was split across
// - sha256sum: sha256sum of the srcFile before splitting
// - err: any errors encountered
func SplitFile(srcPath string, chunkSizeBytes int) (err error) {
	var fileNames []string
	var sha256sum string
	hash := sha256.New()

	// Set buffer size to some multiple of 4096 KiB for modern file system cluster sizes
	bufferSize := 16 * 1024 * 1024 // 16 MiB
	// if chunkSizeBytes is less than bufferSize, use chunkSizeBytes as bufferSize for simplicity
	if chunkSizeBytes < bufferSize {
		bufferSize = chunkSizeBytes
	}
	buf := make([]byte, bufferSize)

	// get file size
	fi, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	fileSize := fi.Size()

	// start progress bar
	title := fmt.Sprintf("[0/%d] MB bytes written", fileSize/1000/1000)
	progressBar := message.NewProgressBar(fileSize, title)
	defer progressBar.Close()

	// open srcFile
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// create file path starting from part 001
	path := fmt.Sprintf("%s.part001", srcPath)
	chunkFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, helpers.ReadAllWriteUser)
	if err != nil {
		return err
	}
	fileNames = append(fileNames, path)
	defer chunkFile.Close()

	// setup counter for tracking how many bytes are left to write to file
	chunkBytesRemaining := chunkSizeBytes
	// Loop over the tarball hashing as we go and breaking it into chunks based on the chunkSizeBytes
	for {
		bytesRead, err := srcFile.Read(buf)

		if err != nil {
			if err == io.EOF {
				// At end of file, break out of loop
				break
			}
			return err
		}

		// Pass data to hash
		hash.Write(buf[0:bytesRead])

		// handle if we should split the data between two chunks
		if chunkBytesRemaining < bytesRead {
			// write the remaining chunk size to file
			_, err := chunkFile.Write(buf[0:chunkBytesRemaining])
			if err != nil {
				return err
			}
			err = chunkFile.Close()
			if err != nil {
				return err
			}

			// create new file
			path = fmt.Sprintf("%s.part%03d", srcPath, len(fileNames)+1)
			chunkFile, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY, helpers.ReadAllWriteUser)
			if err != nil {
				return err
			}
			fileNames = append(fileNames, path)
			defer chunkFile.Close()

			// write to new file where we left off
			_, err = chunkFile.Write(buf[chunkBytesRemaining:bytesRead])
			if err != nil {
				return err
			}

			// set chunkBytesRemaining considering how many bytes are already written to new file
			chunkBytesRemaining = chunkSizeBytes - (bufferSize - chunkBytesRemaining)
		} else {
			_, err := chunkFile.Write(buf[0:bytesRead])
			if err != nil {
				return err
			}
			chunkBytesRemaining = chunkBytesRemaining - bytesRead
		}

		// update progress bar
		progressBar.Add(bufferSize)
		title := fmt.Sprintf("[%d/%d] MB bytes written", progressBar.GetCurrent()/1000/1000, fileSize/1000/1000)
		progressBar.Updatef(title)
	}
	srcFile.Close()
	_ = os.RemoveAll(srcPath)

	// calculate sha256 sum
	sha256sum = fmt.Sprintf("%x", hash.Sum(nil))

	// Marshal the data into a json file.
	jsonData, err := json.Marshal(types.ZarfSplitPackageData{
		Count:     len(fileNames),
		Bytes:     fileSize,
		Sha256Sum: sha256sum,
	})
	if err != nil {
		return fmt.Errorf("unable to marshal the split package data: %w", err)
	}

	// write header file
	path = fmt.Sprintf("%s.part000", srcPath)
	if err := os.WriteFile(path, jsonData, helpers.ReadAllWriteUser); err != nil {
		return fmt.Errorf("unable to write the file %s: %w", path, err)
	}
	fileNames = append(fileNames, path)
	progressBar.Successf("Package split across %d files", len(fileNames))

	return nil
}
