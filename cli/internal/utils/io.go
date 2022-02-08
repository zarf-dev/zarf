package utils

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"

	"github.com/otiai10/copy"
	"github.com/sirupsen/logrus"
)

var TempPathPrefix = "zarf-"

func MakeTempDir() string {
	tmp, err := ioutil.TempDir("", TempPathPrefix)
	logContext := logrus.WithField("path", tmp)
	logContext.Info("Creating temp path")

	if err != nil {
		logContext.Debug(err)
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
		logrus.Debug(err)
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
		logContext.Debug(err)
		logContext.Fatal("Unable to create the file to write the contents")
	}

	_, err = f.Write(data)
	if err != nil {
		_ = f.Close()
		logContext.Debug(err)
		logContext.Fatal("Unable to write the file contents")
	}

	err = f.Close()
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Error saving file")
	}

}

func ReplaceText(path string, old string, new string) {
	logContext := logrus.WithField("path", path)
	input, err := ioutil.ReadFile(path)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to load the given file")
	}

	output := bytes.Replace(input, []byte(old), []byte(new), -1)

	if err = ioutil.WriteFile(path, output, 0600); err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to update the given file")
	}
}

// RecursiveFileList walks a path with an optional regex pattern and returns a slice of file paths
func RecursiveFileList(root string, pattern *regexp.Regexp) []string {
	var files []string

	err := filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				if pattern != nil {
					if len(pattern.FindStringIndex(path)) > 0 {
						files = append(files, path)
					}
				} else {
					files = append(files, path)
				}
			}
			return nil
		})

	if err != nil {
		logrus.Debug(err)
		logrus.WithField("path", root).Fatal("Unable to complete directory walking")
	}

	return files
}

func CreateFilePath(destination string) {
	parentDest := path.Dir(destination)
	err := CreateDirectory(parentDest, 0700)
	if err != nil {
		logrus.Debug(err)
		logrus.WithField("path", parentDest).Fatal("Unable to create the destination path")
	}
}

func CreatePathAndCopy(source string, destination string) {
	logContext := logrus.WithFields(logrus.Fields{
		"Source":      source,
		"Destination": destination,
	})

	logContext.Info("Copying file")

	CreateFilePath(destination)

	// Copy the asset
	err := copy.Copy(source, destination)
	if err != nil {
		logContext.Debug(err)
		logContext.Fatal("Unable to copy the contents of the asset")
	}
}
