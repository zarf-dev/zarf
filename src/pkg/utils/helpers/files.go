// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/mholt/archiver/v3"
)

// FindAndCopyFileFromArchive inspects an archive file, and copies the target file to the destination
func FindAndCopyFileFromArchive(archivePath, targetFile, destinationDir string) error {
	err := archiver.Walk(archivePath, func(f archiver.File) error {
		if strings.HasSuffix(f.Name(), targetFile) {
			// read the file in the compressed file
			data, err := io.ReadAll(f)
			if err != nil {
				return err
			}

			// Create or open the destination file for writing
			destinationPath := filepath.Join(destinationDir, filepath.Base(f.Name()))
			destinationFile, err := os.Create(destinationPath)
			if err != nil {
				return err
			}
			defer destinationFile.Close()

			// Write the data to the destination file
			_, err = destinationFile.Write(data)
			if err != nil {
				return err
			}

		}
		// Remove the compressed file
		err := os.Remove(archivePath)
		if err != nil {
			return fmt.Errorf(lang.ErrRemoveFile, archivePath, err.Error())
		}
		return nil
	})

	return err
}
