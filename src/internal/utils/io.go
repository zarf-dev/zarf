package utils

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/otiai10/copy"
)

const dotCharacter = 46

var TempPathPrefix = "zarf-"

func MakeTempDir(tmpDir string) (string, error) {
	tmp, err := os.MkdirTemp(tmpDir, TempPathPrefix)
	message.Debugf("Creating temp path %s", tmp)
	return tmp, err
}

// VerifyBinary returns true if binary is available
func VerifyBinary(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

// CreateDirectory creates a directory for the given path and file mode
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

func ListDirectories(directory string) ([]string, error) {
	var directories []string
	paths, err := os.ReadDir(directory)
	if err != nil {
		return directories, fmt.Errorf("unable to load the directory %s: %w", directory, err)
	}

	for _, entry := range paths {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(directory, entry.Name()))
		}
	}

	return directories, nil
}

func WriteFile(path string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create the file at %s to write the contents: %w", path, err)
	}

	_, err = f.Write(data)
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("unable to write the file at %s contents:%w", path, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("error saving file %s: %w", path, err)
	}

	return nil
}

// ReplaceTextTemplate loads a file from a given path, replaces text in it and writes it back in place
func ReplaceTextTemplate(path string, mappings map[string]string) {
	text, err := os.ReadFile(path)
	if err != nil {
		message.Fatalf(err, "Unable to load %s", path)
	}

	for template, value := range mappings {
		text = bytes.ReplaceAll(text, []byte(template), []byte(value))
	}

	if err = os.WriteFile(path, text, 0600); err != nil {
		message.Fatalf(err, "Unable to update %s", path)
	}
}

// RecursiveFileList walks a path with an optional regex pattern and returns a slice of file paths
func RecursiveFileList(dir string, pattern *regexp.Regexp) (files []string, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		// Skip hidden directories
		if d.IsDir() && d.Name()[0] == dotCharacter {
			return filepath.SkipDir
		}

		// Return errors
		if err != nil {
			return err
		}

		if !d.IsDir() {
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
	return files, err
}

func CreateFilePath(destination string) error {
	parentDest := path.Dir(destination)
	return CreateDirectory(parentDest, 0700)
}

func CreatePathAndCopy(source string, destination string) error {
	if err := CreateFilePath(destination); err != nil {
		return err
	}

	if err := copy.Copy(source, destination); err != nil {
		return err
	}

	return nil
}

// GetFinalExecutablePath returns the absolute path to the zarf executable, following any symlinks along the way
func GetFinalExecutablePath() (string, error) {
	message.Debug("utils.GetExecutablePath()")

	binaryPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// In case the binary is symlinked somewhere else, get the final destination!!
	linkedPath, err := filepath.EvalSymlinks(binaryPath)
	return linkedPath, err
}
