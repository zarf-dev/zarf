// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides shared filesystem, URL, and value helpers.
package helpers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/otiai10/copy"
)

const (
	// ReadUser is used for any internal file to be read only
	ReadUser = 0400
	// ReadWriteUser is used for any internal file not normally used by the end user or containing sensitive data
	ReadWriteUser = 0600
	// ReadAllWriteUser is used for any non sensitive file intended to be consumed by the end user
	ReadAllWriteUser = 0644
	// ReadWriteExecuteUser is used for any directory or executable not normally used by the end user or containing sensitive data
	ReadWriteExecuteUser = 0700
	// ReadExecuteAllWriteUser is used for any non sensitive directory or executable intended to be consumed by the end user
	ReadExecuteAllWriteUser = 0755
)

// CreateDirectory creates a directory for the given path and file mode.
func CreateDirectory(path string, mode os.FileMode) error {
	if InvalidPath(path) {
		return os.MkdirAll(path, mode)
	}
	return nil
}

// CreateParentDirectory creates the parent directory for the given file path.
func CreateParentDirectory(destination string) error {
	return CreateDirectory(filepath.Dir(destination), ReadWriteExecuteUser)
}

// CreatePathAndCopy creates the parent directory for the given file path and copies the source file to the destination.
func CreatePathAndCopy(source, destination string) error {
	if err := CreateParentDirectory(destination); err != nil {
		return err
	}
	if err := copy.Copy(source, destination); err != nil {
		return err
	}
	if InvalidPath(destination) {
		file, err := os.Create(destination)
		if err != nil {
			return err
		}
		return file.Close()
	}
	return nil
}

// InvalidPath checks if the given path is valid (if it is a permissions error it is there we just don't have access)
func InvalidPath(path string) bool {
	_, err := os.Stat(path)
	return !os.IsPermission(err) && err != nil
}

// RecursiveFileList walks a path with an optional regex pattern and returns a slice of file paths.
// If skipHidden is true, hidden directories will be skipped.
func RecursiveFileList(dir string, pattern *regexp.Regexp, skipHidden bool) (files []string, err error) {
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && skipHidden && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		if !entry.Type().IsRegular() || (pattern != nil && !pattern.MatchString(path)) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// ReadFileByChunks reads a file into multiple chunks by the given size.
func ReadFileByChunks(path string, chunkSize int) (chunks [][]byte, sha256sum string, err error) {
	if chunkSize <= 0 {
		return nil, "", fmt.Errorf("chunk size must be greater than zero")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	for len(data) > 0 {
		size := min(len(data), chunkSize)
		chunks = append(chunks, data[:size])
		data = data[size:]
	}
	return chunks, fmt.Sprintf("%x", sum), nil
}

// IsDir returns true if the given path is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	return err == nil && info.IsDir()
}

// GetSHA256OfFile returns the SHA256 hash of the provided file.
func GetSHA256OfFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck
	return GetSHA256Hash(file)
}

// SHAsMatch returns an error if the SHA256 hash of the provided file does not match the expected hash.
func SHAsMatch(path, expected string) error {
	actual, err := GetSHA256OfFile(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("expected sha256 of %s to be %s, found %s", path, expected, actual)
	}
	return nil
}

// GetSHA256Hash returns the computed SHA256 Sum of a given file
func GetSHA256Hash(data io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, data); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// IsTextFile returns true if path is a text file, false otherwise. It might
// return an error if the file cannot be read.
func IsTextFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close() //nolint:errcheck
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	for _, offset := range []int64{0, max(0, info.Size()-512)} {
		buffer := make([]byte, 512)
		count, readErr := file.ReadAt(buffer, offset)
		if readErr != nil && readErr != io.EOF {
			return false, readErr
		}
		mimeType := http.DetectContentType(buffer[:count])
		if !strings.HasPrefix(mimeType, "text/") && !strings.Contains(mimeType, "json") && !strings.Contains(mimeType, "xml") {
			return false, nil
		}
	}
	return true, nil
}
