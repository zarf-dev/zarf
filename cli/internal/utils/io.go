package utils

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/defenseunicorns/zarf/cli/internal/log"
	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

var TempPathPrefix = "zarf-"

func MakeTempDir() string {
	tmp, err := ioutil.TempDir("", TempPathPrefix)
	logContext := log.Logger.WithField("path", tmp)
	logContext.Info("Creating temp path")

	if err != nil {
		logContext.Fatal("Unable to create temp directory")
	}
	return tmp
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
	var directories []string
	paths, err := os.ReadDir(directory)
	if err != nil {
		log.Logger.WithField("path", directory).Fatal("Unable to load the directory")
	}

	for _, entry := range paths {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(directory, entry.Name()))
		}
	}

	return directories
}

func WriteFile(path string, data []byte) {

	logContext := log.Logger.WithField("path", path)

	f, err := os.Create(path)
	if err != nil {
		logContext.Fatal("Unable to create the file to write the contents")
	}

	_, err = f.Write(data)
	if err != nil {
		logContext.Fatal("Unable to write the file contents")
		_ = f.Close()
	}

	err = f.Close()
	if err != nil {
		logContext.Fatal("Error saving file")
	}

}

func ReplaceText(path string, old string, new string) {
	logContext := log.Logger.WithField("path", path)
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
	var files []string

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
		log.Logger.WithField("path", root).Fatal("Unable to complete directory walking")
	}

	return files
}

func CreateFilePath(destination string) {
	parentDest := path.Dir(destination)
	err := CreateDirectory(parentDest, 0700)
	if err != nil {
		log.Logger.WithField("path", parentDest).Fatal("Unable to create the destination path")
	}
}

func CreatePathAndCopy(source string, destination string) {
	logContext := log.Logger.WithFields(logrus.Fields{
		"Source":      source,
		"Destination": destination,
	})

	logContext.Info("Copying file")

	CreateFilePath(destination)

	// Copy the asset
	err := copy.Copy(source, destination)
	if err != nil {
		logContext.Fatal("Unable to copy the contents of the asset")
	}
}
