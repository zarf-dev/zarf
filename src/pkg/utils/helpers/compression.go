package helpers

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

func ExtractTarGz(filenameWithPath string) error {
	// Open the .tar.gz file for reading
	file, err := os.Open(filenameWithPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}

		// Construct the target file path
		targetPath := filepath.Join(filepath.Dir(filenameWithPath), header.Name)

		// Create directories if needed
		if header.Typeflag == tar.TypeDir {
			err := os.MkdirAll(targetPath, os.ModePerm)
			if err != nil {
				return err
			}
			continue
		}

		// Create the target file
		fileWriter, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer fileWriter.Close()

		// Copy the file contents
		_, err = io.Copy(fileWriter, tarReader)
		if err != nil {
			return err
		}
	}

	return nil
}
