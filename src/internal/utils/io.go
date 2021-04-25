package utils

import (
	"embed"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"os"
	"os/exec"
	"strings"
)

//go:embed assets
var assets embed.FS

// VerifyBinary returns true if binary is available
func VerifyBinary(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

// CreateDirectory
func CreateDirectory(path string, mode os.FileMode) error {
	if InvalidPath(path) {
		return os.Mkdir(path, mode)
	}

	return nil
}

// Invalid checks if the given path exists
func InvalidPath(dir string) bool {
	_, err := os.Stat(dir)
	return os.IsNotExist(err)
}

func ReadAsset(path string) ([]byte, error) {
	return assets.ReadFile(path)
}

// WriteAssets writes given files to the filesystem from the binary storage
func WriteAssets(source string, destination string) {

	logContext := log.WithFields(log.Fields{
		"source":      source,
		"destination": destination,
	})

	logContext.Info("Expanding resources")

	fs.WalkDir(assets, source, func(path string, d fs.DirEntry, err error) error {

		if err != nil {
			logContext.Fatal("Error reading embedded resource")
			return err
		}

		fullPath := strings.Replace(path, source, destination, 1)
		fileLogContext := log.WithField("path", fullPath)

		if d.IsDir() {
			fileLogContext.Info("Created directory")
			errDir := os.MkdirAll(fullPath, 0700)
			if errDir != nil {
				fileLogContext.Fatal("Error creating directory")
				return err
			}
		} else {
			fileLogContext.Info("Writing file")

			file, err := os.Create(fullPath)
			if err != nil {
				fileLogContext.Fatal("Error creating file")
			}

			defer file.Close()

			fileData, _ := ReadAsset(path)
			_, err = file.Write(fileData)
			if err != nil {
				fileLogContext.Fatal("Error writing to file")
			}

		}

		return nil
	})
}
