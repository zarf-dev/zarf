// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

// Package helpers provides common helper functions for Go.
package helpers

import (
	"archive/tar"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/otiai10/copy"
)

const dotCharacter = 46

// CreateDirectory creates a directory for the given path and file mode.
func CreateDirectory(path string, mode os.FileMode) error {
	if InvalidPath(path) {
		return os.MkdirAll(path, mode)
	}
	return nil
}

// CreateFile creates an empty file at the given path.
func CreateFile(filepath string) error {
	if InvalidPath(filepath) {
		f, err := os.Create(filepath)
		f.Close()
		return err
	}

	return nil
}

// InvalidPath checks if the given path is valid (if it is a permissions error it is there we just don't have access)
func InvalidPath(path string) bool {
	_, err := os.Stat(path)
	return !os.IsPermission(err) && err != nil
}

// ListDirectories returns a list of directories in the given directory.
func ListDirectories(directory string) ([]string, error) {
	var directories []string
	paths, err := os.ReadDir(directory)
	if err != nil {
		return directories, fmt.Errorf("unable to load the directory %s: %w", directory, err)
	}

	for _, entry := range paths {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(directory, entry.Name()))
		}
	}

	return directories, nil
}

// RecursiveFileList walks a path with an optional regex pattern and returns a slice of file paths.
// If skipHidden is true, hidden directories will be skipped.
func RecursiveFileList(dir string, pattern *regexp.Regexp, skipHidden bool) (files []string, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		// Return errors
		if err != nil {
			return err
		}

		info, err := d.Info()

		if err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			if pattern != nil {
				if len(pattern.FindStringIndex(path)) > 0 {
					files = append(files, path)
				}
			} else {
				files = append(files, path)
			}
			// Skip hidden directories
		} else if skipHidden && IsHidden(d.Name()) {
			return filepath.SkipDir
		}

		return nil
	})
	return files, err
}

// CreateParentDirectory creates the parent directory for the given file path.
func CreateParentDirectory(destination string) error {
	parentDest := filepath.Dir(destination)
	return CreateDirectory(parentDest, ReadWriteExecuteUser)
}

// CreatePathAndCopy creates the parent directory for the given file path and copies the source file to the destination.
func CreatePathAndCopy(source string, destination string) error {
	if err := CreateParentDirectory(destination); err != nil {
		return err
	}

	// Copy all the source data into the destination location
	if err := copy.Copy(source, destination); err != nil {
		return err
	}

	// If the path doesn't exist yet then this is an empty file and we should create it
	return CreateFile(destination)
}

// ReadFileByChunks reads a file into multiple chunks by the given size.
func ReadFileByChunks(path string, chunkSizeBytes int) (chunks [][]byte, sha256sum string, err error) {
	var file []byte

	// Open the created archive for io.Copy
	if file, err = os.ReadFile(path); err != nil {
		return chunks, sha256sum, err
	}

	//Calculate the sha256sum of the file before we split it up
	sha256sum = fmt.Sprintf("%x", sha256.Sum256(file))

	// Loop over the tarball breaking it into chunks based on the payloadChunkSize
	for len(file) != 0 {
		// don't bust slice length
		if len(file) < chunkSizeBytes {
			chunkSizeBytes = len(file)
		}

		chunks = append(chunks, file[0:chunkSizeBytes])
		file = file[chunkSizeBytes:]
	}

	return chunks, sha256sum, nil
}

// IsTrashBin checks if the given directory path corresponds to an operating system's trash bin.
func IsTrashBin(dirPath string) bool {
	dirPath = filepath.Clean(dirPath)

	// Check if the directory path matches a Linux trash bin
	if strings.HasSuffix(dirPath, "/Trash") || strings.HasSuffix(dirPath, "/.Trash-1000") {
		return true
	}

	// Check if the directory path matches a macOS trash bin
	if strings.HasSuffix(dirPath, "./Trash") || strings.HasSuffix(dirPath, "/.Trashes") {
		return true
	}

	// Check if the directory path matches a Windows trash bin
	if strings.HasSuffix(dirPath, "\\$RECYCLE.BIN") {
		return true
	}

	return false
}

// IsHidden returns true if the given file name starts with a dot.
func IsHidden(name string) bool {
	return name[0] == dotCharacter
}

// GetDirSize walks through all files and directories in the provided path and returns the total size in bytes.
func GetDirSize(path string) (int64, error) {
	dirSize := int64(0)

	// Walk all files in the path
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			dirSize += info.Size()
		}
		return nil
	})

	return dirSize, err
}

// IsDir returns true if the given path is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// GetSHA256OfFile returns the SHA256 hash of the provided file.
func GetSHA256OfFile(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return GetSHA256Hash(f)
}

// SHAsMatch returns an error if the SHA256 hash of the provided file does not match the expected hash.
func SHAsMatch(path, expected string) error {
	sha, err := GetSHA256OfFile(path)
	if err != nil {
		return err
	}
	if sha != expected {
		return fmt.Errorf("expected sha256 of %s to be %s, found %s", path, expected, sha)
	}
	return nil
}

// CreateReproducibleTarballFromDir creates a tarball from a directory with stripped headers
func CreateReproducibleTarballFromDir(dirPath, dirPrefix, tarballPath string) error {
	tb, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("error creating tarball: %w", err)
	}
	defer tb.Close()

	tw := tar.NewWriter(tb)
	defer tw.Close()

	// Walk through the directory and process each file
	return filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
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
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("error writing file to tarball: %w", err)
			}
		}

		return nil
	})
}
