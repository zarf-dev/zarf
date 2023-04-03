// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/otiai10/copy"
)

const (
	dotCharacter  = 46
	tmpPathPrefix = "zarf-"
)

type TextTemplate struct {
	Sensitive  bool
	AutoIndent bool
	Value      string
}

// MakeTempDir creates a temp directory with the given prefix.
func MakeTempDir(tmpDir string) (string, error) {
	// Create the base tmp directory if it is specified.
	if tmpDir != "" {
		if err := CreateDirectory(tmpDir, 0700); err != nil {
			return "", err
		}
	}

	tmp, err := os.MkdirTemp(tmpDir, tmpPathPrefix)
	message.Debugf("Using temp path: '%s'", tmp)
	return tmp, err
}

// VerifyBinary returns true if binary is available.
func VerifyBinary(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

// CreateDirectory creates a directory for the given path and file mode.
func CreateDirectory(path string, mode os.FileMode) error {
	if InvalidPath(path) {
		return os.MkdirAll(path, mode)
	}
	return nil
}

// InvalidPath checks if the given path exists.
func InvalidPath(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// ListDirectories returns a list of directories in the given directory.
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

// WriteFile writes the given data to the given path.
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

// ReplaceTextTemplate loads a file from a given path, replaces text in it and writes it back in place.
func ReplaceTextTemplate(path string, mappings map[string]*TextTemplate, deprecations map[string]string, templateRegex string) error {
	textFile, err := os.Open(path)
	if err != nil {
		return err
	}

	// This regex takes a line and parses the text before and after a discovered template: https://regex101.com/r/ilUxAz/1
	regexTemplateLine := regexp.MustCompile(fmt.Sprintf("(?P<preTemplate>.*?)(?P<template>%s)(?P<postTemplate>.*)", templateRegex))

	fileScanner := bufio.NewScanner(textFile)
	fileScanner.Split(bufio.ScanLines)

	text := ""

	for fileScanner.Scan() {
		line := fileScanner.Text()

		for {
			matches := regexTemplateLine.FindStringSubmatch(line)

			// No template left on this line so move on
			if len(matches) == 0 {
				text += fmt.Sprintln(line)
				break
			}

			preTemplate := matches[regexTemplateLine.SubexpIndex("preTemplate")]
			templateKey := matches[regexTemplateLine.SubexpIndex("template")]

			_, present := deprecations[templateKey]
			if present {
				message.Warnf("This Zarf Package uses a deprecated variable: '%s' changed to '%s'.  Please notify your package creator for an update.", templateKey, deprecations[templateKey])
			}

			template := mappings[templateKey]

			// Check if the template is nil (present), use the original templateKey if not (so that it is not replaced).
			value := templateKey
			if template != nil {
				value = template.Value

				// Check if the value is autoIndented and add the correct spacing
				if template.AutoIndent {
					indent := fmt.Sprintf("\n%s", strings.Repeat(" ", len(preTemplate)))
					value = strings.ReplaceAll(value, "\n", indent)
				}
			}

			// Add the processed text and continue processing the line
			text += fmt.Sprintf("%s%s", preTemplate, value)
			line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
		}
	}

	textFile.Close()

	if err = os.WriteFile(path, []byte(text), 0600); err != nil {
		return err
	}

	return nil
}

// RecursiveFileList walks a path with an optional regex pattern and returns a slice of file paths.
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

// CreateFilePath creates the parent directory for the given file path.
func CreateFilePath(destination string) error {
	parentDest := filepath.Dir(destination)
	return CreateDirectory(parentDest, 0700)
}

// CreatePathAndCopy creates the parent directory for the given file path and copies the source file to the destination.
func CreatePathAndCopy(source string, destination string) error {
	if err := CreateFilePath(destination); err != nil {
		return err
	}

	return copy.Copy(source, destination)
}

// GetFinalExecutablePath returns the absolute path to the Zarf executable, following any symlinks along the way.
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

// SplitFile splits a file into multiple parts by the given size.
func SplitFile(path string, chunkSizeBytes int) (chunks [][]byte, sha256sum string, err error) {
	var file []byte

	// Open the created archive for io.Copy
	if file, err = os.ReadFile(path); err != nil {
		return chunks, sha256sum, err
	}

	//Calculate the sha256sum of the file before we split it up
	sha256sum = fmt.Sprintf("%x", sha256.Sum256(file))

	// Loop over the tarball breaking it into chunks based on the payloadChunkSize
	for {
		if len(file) == 0 {
			break
		}

		// don't bust slice length
		if len(file) < chunkSizeBytes {
			chunkSizeBytes = len(file)
		}

		chunks = append(chunks, file[0:chunkSizeBytes])
		file = file[chunkSizeBytes:]
	}

	return chunks, sha256sum, nil
}

// IsTextFile returns true if the given file is a text file.
func IsTextFile(path string) (bool, error) {
	// Open the file
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close() // Make sure to close the file when we're done

	// Read the first 512 bytes of the file
	data := make([]byte, 512)
	n, err := f.Read(data)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Use http.DetectContentType to determine the MIME type of the file
	mimeType := http.DetectContentType(data[:n])

	// Check if the MIME type indicates that the file is text
	hasText := strings.HasPrefix(mimeType, "text/")
	hasJSON := strings.Contains(mimeType, "json")
	hasXML := strings.Contains(mimeType, "xml")

	return hasText || hasJSON || hasXML, nil
}

// GetDirSize walks through all files and directories in the provided path and returns the total size in bytes.
func GetDirSize(path string) (int64, error) {
	dirSize := int64(0)

	// Walk through all files in the path
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			dirSize += info.Size()
		}
		return nil
	})

	return dirSize, err
}

// GetSHA256OfFile returns the SHA256 hash of the provided file.
func GetSHA256OfFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
