package helpers

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
)

func KeepOnlyFiles(directory string, fileNames []string) error {
	files, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	filesToKeep := make(map[string]bool)
	for _, fileName := range fileNames {
		filesToKeep[fileName] = true
	}

	for _, file := range files {
		filePath := filepath.Join(directory, file.Name())
		if !filesToKeep[file.Name()] {
			err := os.RemoveAll(filePath)
			if err != nil {
				return err
			}
			fmt.Println("Deleted:", filePath)
		}
	}

	return nil
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
