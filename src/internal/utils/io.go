package utils

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/mholt/archiver/v3"
	log "github.com/sirupsen/logrus"
)

var tempDestination string

func extractArchive() {

	tmp, err := ioutil.TempDir("", "shift-package")
	log.WithField("path", tmp).Info("Extracting assets")

	if err != nil {
		log.WithField("path", tmp).Fatal("Unable to create temp directory")
	}

	err = archiver.Unarchive("shift-package.tar.zst", tmp)
	if err != nil {
		log.Fatal("Unable to extract the shift-package.tar.zst contents")
	}

	tempDestination = tmp
}

// VerifyBinary returns true if binary is available
func VerifyBinary(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

// CreateDirectory
func CreateDirectory(path string, mode os.FileMode) error {
	if InvalidPath(path) {
		return os.MkdirAll(path, mode)
	}

	return nil
}

// InvalidPath checks if the given path exists
func InvalidPath(dir string) bool {
	_, err := os.Stat(dir)
	return os.IsNotExist(err)
}

func PlaceAsset(source string, destination string) {

	if tempDestination == "" {
		extractArchive()
	}

	// Prepend the temp dir path
	sourcePath := tempDestination + "/" + source
	parentDest := path.Dir(destination)
	logContext := log.WithFields(log.Fields{
		"Source":      sourcePath,
		"Destination": destination,
	})

	logContext.Info("Placing asset")
	err := CreateDirectory(parentDest, 0700)
	if err != nil {
		logContext.Fatal("Unable to create the required destination path")
	}

	// Move the asset
	err = os.Rename(sourcePath, destination)
	if err != nil {
		logContext.Fatal("Unable to extract a required asset")
	}
}
