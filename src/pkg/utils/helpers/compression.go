// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func KeepOnlyFile(directory, filename string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.Name() != filename {
			filePath := filepath.Join(directory, file.Name())
			err := os.RemoveAll(filePath)
			if err != nil {
				return err
			}
			fmt.Println("Deleted:", filePath)
		}
	}

	return nil
}

func SupportedCompressionFormat(filename string) bool {
	supportedFormats := []string{".tar.gz", ".br", ".bz2", ".zip", ".lz4", ".sz", ".xz", ".zz", ".zst"}

	for _, format := range supportedFormats {
		if strings.HasSuffix(filename, format) {
			return true
		}
	}
	return false
}

func GetDirFromFilename(target string) string {
	return filepath.Dir(target)
}
func RenamePathWithFilename(target, fileName string) (string, error) {
	dir := filepath.Dir(target)
	newPath := filepath.Join(dir, fileName)
	return newPath, nil
}
func ExtractFilenameFromURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	filename := path.Base(parsedURL.Path)
	return filename, nil
}
