package utils

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/mholt/archiver/v3"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

var TempDestination string
var ArchivePath = "zarf-initialize.tar.zst"
var TempPathPrefix = "zarf-"

func eraseTempAssets() {
	files, _ := filepath.Glob("/tmp/" + TempPathPrefix + "*")
	for _, path := range files {
		err := os.RemoveAll(path)
		logContext := logrus.WithField("path", path)
		if err != nil {
			logContext.Warn("Unable to purge temporary path")
		} else {
			logContext.Info("Purging old temp files")
		}
	}
}

func extractArchive() {
	eraseTempAssets()

	tmp := MakeTempDir()

	err := Decompress(ArchivePath, tmp)
	if err != nil {
		logrus.WithField("source", ArchivePath).Fatal("Unable to extract the archive contents")
	}

	TempDestination = tmp
}

func MakeTempDir() string {
	tmp, err := ioutil.TempDir("", TempPathPrefix)
	logContext := logrus.WithField("path", tmp)
	logContext.Info("Creating temp path")

	if err != nil {
		logContext.Fatal("Unable to create temp directory")
	}
	return tmp
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
		logrus.WithField("path", path).Warn("Unable to find matching files")
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
		logrus.WithField("path", directory).Fatal("Unable to load the directory")
	}

	for _, entry := range paths {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(directory, entry.Name()))
		}
	}

	return directories
}

func WriteFile(path string, data []byte) {

	logContext := logrus.WithField("path", path)

	f, err := os.Create(path)
	if err != nil {
		logContext.Fatal("Unable to create the file to write the contents")
	}

	_, err = f.Write(data)
	if err != nil {
		logContext.Fatal("Unable to write the file contents")
		f.Close()
	}

	err = f.Close()
	if err != nil {
		logContext.Fatal("Error saving file")
	}

}

func ReplaceText(path string, old string, new string) {
	logContext := logrus.WithField("path", path)
	input, err := ioutil.ReadFile(path)
	if err != nil {
		logContext.Fatal("Unable to load the given file")
	}

	output := bytes.Replace(input, []byte(old), []byte(new), -1)

	if err = ioutil.WriteFile(path, output, 0600); err != nil {
		logContext.Fatal("Unable to update the given file")
	}
}

func RecursiveFileList(root string) []string {
	files := []string{}

	err := filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})

	if err != nil {
		logrus.WithField("path", root).Fatal("Unable to complete directory walking")
	}

	return files
}

func Compress(sources []string, destination string) error {
	return archiver.Archive(sources, destination)
}

func Decompress(source string, destination string) error {
	return archiver.Unarchive(source, destination)
}

func PlaceAsset(source string, destination string) {
	sourcePath := AssetPath(source)
	CreatePathAndCopy(sourcePath, destination)
}

func CreatePathAndCopy(source string, destination string) {
	parentDest := path.Dir(destination)
	logContext := logrus.WithFields(logrus.Fields{
		"Source":      source,
		"Destination": destination,
	})

	logContext.Info("Placing asset")
	err := CreateDirectory(parentDest, 0700)
	if err != nil {
		logContext.Fatal("Unable to create the required destination path")
	}

	// Copy the asset
	err = copy.Copy(source, destination)

	if err != nil {
		logContext.Fatal("Unable to copy the contens of the asset")
	}
}
