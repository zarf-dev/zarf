package utils

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	log "github.com/sirupsen/logrus"
)

var TempDestination string
var ArchivePath = "zarf-initialize.tar.zst"
var TempPathPrefix = "zarf-"

func eraseTempAssets() {
	files, _ := filepath.Glob("/tmp/" + TempPathPrefix + "*")
	for _, path := range files {
		err := os.RemoveAll(path)
		logContext := log.WithField("path", path)
		if err != nil {
			logContext.Warn("Unable to purge temporary path")
		} else {
			logContext.Info("Purging old temp files")
		}
	}
}

func extractArchive() {
	eraseTempAssets()

	tmp, err := ioutil.TempDir("", TempPathPrefix)
	logContext := log.WithFields(log.Fields{
		"source":      ArchivePath,
		"destination": tmp,
	})

	logContext.Info("Extracting assets")

	if err != nil {
		logContext.Fatal("Unable to create temp directory")
	}

	err = Decompress(ArchivePath, tmp)
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

func ListDirectories(directory string) []string {
	directories := []string{}
	paths, err := os.ReadDir(directory)
	if err != nil {
		log.WithField("path", directory).Fatal("Unable to load the directory")
	}

	for _, entry := range paths {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(directory, entry.Name()))
		}
	}

	return directories
}

func Compress(sources []string, destination string) error {
	return archiver.Archive(sources, destination)
}

func Decompress(source string, destination string) error {
	return archiver.Unarchive(source, destination)
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

	// Copy the asset
	err = copy.Copy(sourcePath, destination)

	if err != nil {
		logContext.Fatal("Unable to copy the contens of the asset")
	}
}
