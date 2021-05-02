package utils

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/mholt/archiver/v3"
	log "github.com/sirupsen/logrus"
)

var TempDestination string
var ArchivePath = "shift-pack.tar.zst"

func extractArchive() {
	tmp, err := ioutil.TempDir("", "shift-pack-")
	logContext := log.WithFields(log.Fields{
		"source":      ArchivePath,
		"destination": tmp,
	})
	
	logContext.Info("Extracting assets")

	if err != nil {
		logContext.Fatal("Unable to create temp directory")
	}

	err = archiver.Unarchive(ArchivePath, tmp)
	if err != nil {
		logContext.Fatal("Unable to extract the arhive contents")
	}

	TempDestination = tmp
}

func AssetPath(partial string) string {
	if TempDestination == "" {
		extractArchive()
	}
	return TempDestination + "/" + partial
}

// AssetList given a path, return a glob matching all files in the archive
func AssetList(partial string) []string {
	path := AssetPath(partial)
	matches, err := filepath.Glob(path)
	if err != nil {
		log.WithField("path", path).Warn("Unable to find matching files")
	}
	return matches
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
func InvalidPath(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func PlaceAsset(source string, destination string) {

	// Prepend the temp dir path
	sourcePath := AssetPath(source)
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
