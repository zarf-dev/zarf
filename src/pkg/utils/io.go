// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"archive/tar"
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/otiai10/copy"
)

const (
	dotCharacter  = 46
	tmpPathPrefix = "zarf-"
)

// TextTemplate represents a value to be templated into a text file.
type TextTemplate struct {
	Sensitive  bool
	AutoIndent bool
	Type       types.VariableType
	Value      string
}

// MakeTempDir creates a temp directory with the zarf- prefix.
func MakeTempDir(basePath string) (string, error) {
	if basePath != "" {
		if err := CreateDirectory(basePath, 0700); err != nil {
			return "", err
		}
	}
	tmp, err := os.MkdirTemp(basePath, tmpPathPrefix)
	message.Debug("Using temporary directory:", tmp)
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

// CreateFile creates an empty file at the given path.
func CreateFile(filepath string) error {
	if InvalidPath(filepath) {
		f, err := os.Create(filepath)
		f.Close()
		return err
	}

	return nil

}

// InvalidPath checks if the given path is valid (if it is a permissions error it is there we just don't have access)
func InvalidPath(path string) bool {
	_, err := os.Stat(path)
	return !os.IsPermission(err) && err != nil
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

	// Set the buffer to 1 MiB to handle long lines (i.e. base64 text in a secret)
	// 1 MiB is around the documented maximum size for secrets and configmaps
	const maxCapacity = 1024 * 1024
	buf := make([]byte, maxCapacity)
	fileScanner.Buffer(buf, maxCapacity)

	// Set the scanner to split on new lines
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

				// Check if the value is a file type and load the value contents from the file
				if template.Type == types.FileVariableType && value != "" {
					if isText, err := IsTextFile(value); err != nil || !isText {
						message.Warnf("Refusing to load a non-text file for templating %s", templateKey)
						line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
						continue
					}

					contents, err := os.ReadFile(value)
					if err != nil {
						message.Warnf("Unable to read file for templating - skipping: %s", err.Error())
						line = matches[regexTemplateLine.SubexpIndex("postTemplate")]
						continue
					}

					value = string(contents)
				}

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

	return os.WriteFile(path, []byte(text), 0600)

}

// RecursiveFileList walks a path with an optional regex pattern and returns a slice of file paths.
// If skipHidden is true, hidden directories will be skipped.
func RecursiveFileList(dir string, pattern *regexp.Regexp, skipHidden bool) (files []string, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {

		// Return errors
		if err != nil {
			return err
		}

		info, err := d.Info()

		if err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			if pattern != nil {
				if len(pattern.FindStringIndex(path)) > 0 {
					files = append(files, path)
				}
			} else {
				files = append(files, path)
			}
			// Skip hidden directories
		} else if skipHidden && IsHidden(d.Name()) {
			return filepath.SkipDir
		}

		return nil
	})
	return files, err
}

// CreateParentDirectory creates the parent directory for the given file path.
func CreateParentDirectory(destination string) error {
	parentDest := filepath.Dir(destination)
	return CreateDirectory(parentDest, 0700)
}

// CreatePathAndCopy creates the parent directory for the given file path and copies the source file to the destination.
func CreatePathAndCopy(source string, destination string) error {
	if err := CreateParentDirectory(destination); err != nil {
		return err
	}

	// Copy all the source data into the destination location
	if err := copy.Copy(source, destination); err != nil {
		return err
	}

	// If the path doesn't exist yet then this is an empty file and we should create it
	return CreateFile(destination)
}

// GetFinalExecutablePath returns the absolute path to the current executable, following any symlinks along the way.
func GetFinalExecutablePath() (string, error) {
	message.Debug("utils.GetExecutablePath()")

	binaryPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// In case the binary is symlinked somewhere else, get the final destination
	linkedPath, err := filepath.EvalSymlinks(binaryPath)
	return linkedPath, err
}

// GetFinalExecutableCommand returns the final path to the Zarf executable including and library prefixes and overrides.
func GetFinalExecutableCommand() (string, error) {
	// In case the binary is symlinked somewhere else, get the final destination
	zarfCommand, err := GetFinalExecutablePath()
	if err != nil {
		return zarfCommand, err
	}

	if config.ActionsCommandZarfPrefix != "" {
		zarfCommand = fmt.Sprintf("%s %s", zarfCommand, config.ActionsCommandZarfPrefix)
	}

	// If a library user has chosen to override config to use system Zarf instead, reset the binary path.
	if config.ActionsUseSystemZarf {
		zarfCommand = "zarf"
	}

	return zarfCommand, err
}

// ReadFileByChunks reads a file into multiple chunks by the given size.
func ReadFileByChunks(path string, chunkSizeBytes int) (chunks [][]byte, sha256sum string, err error) {
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

// SplitFile will take a srcFile path and split it into files based on chunkSizeBytes
// the first file will be a metadata file containing:
// - sha256sum of the original file
// - number of bytes in the original file
// - number of files the srcFile was split into
// SplitFile will delete the original file
//
// Returns:
// - fileNames: list of file paths srcFile was split across
// - sha256sum: sha256sum of the srcFile before splitting
// - err: any errors encountered
func SplitFile(srcFile string, chunkSizeBytes int) (err error) {
	var fileNames []string
	var sha256sum string
	hash := sha256.New()

	// Set buffer size to some multiple of 4096 KiB for modern file system cluster sizes
	bufferSize := 16 * 1024 * 1024 // 16 MiB
	// if chunkSizeBytes is less than bufferSize, use chunkSizeBytes as bufferSize for simplicity
	if chunkSizeBytes < bufferSize {
		bufferSize = chunkSizeBytes
	}
	buf := make([]byte, bufferSize)

	// get file size
	fi, err := os.Stat(srcFile)
	if err != nil {
		return err
	}
	fileSize := fi.Size()

	// start progress bar
	title := fmt.Sprintf("[0/%d] MB bytes written", fileSize/1000/1000)
	progressBar := message.NewProgressBar(fileSize, title)
	defer progressBar.Stop()

	// open file
	file, err := os.Open(srcFile)
	defer file.Close()
	if err != nil {
		return err
	}

	// create file path starting from part 001
	path := fmt.Sprintf("%s.part001", srcFile)
	chunkFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	fileNames = append(fileNames, path)
	defer chunkFile.Close()

	// setup counter for tracking how many bytes are left to write to file
	chunkBytesRemaining := chunkSizeBytes
	// Loop over the tarball hashing as we go and breaking it into chunks based on the chunkSizeBytes
	for {
		bytesRead, err := file.Read(buf)

		if err != nil {
			if err == io.EOF {
				// At end of file, break out of loop
				break
			}
			return err
		}

		// Pass data to hash
		hash.Write(buf[0:bytesRead])

		// handle if we should split the data between two chunks
		if chunkBytesRemaining < bytesRead {
			// write the remaining chunk size to file
			_, err := chunkFile.Write(buf[0:chunkBytesRemaining])
			if err != nil {
				return err
			}

			// create new file
			path = fmt.Sprintf("%s.part%03d", srcFile, len(fileNames)+1)
			chunkFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			fileNames = append(fileNames, path)
			defer chunkFile.Close()

			// write to new file where we left off
			_, err = chunkFile.Write(buf[chunkBytesRemaining:bytesRead])
			if err != nil {
				return err
			}

			// set chunkBytesRemaining considering how many bytes are already written to new file
			chunkBytesRemaining = chunkSizeBytes - (bufferSize - chunkBytesRemaining)
		} else {
			_, err := chunkFile.Write(buf[0:bytesRead])
			if err != nil {
				return err
			}
			chunkBytesRemaining = chunkBytesRemaining - bytesRead
		}

		// update progress bar
		progressBar.Add(bufferSize)
		title := fmt.Sprintf("[%d/%d] MB bytes written", progressBar.GetCurrent()/1000/1000, fileSize/1000/1000)
		progressBar.UpdateTitle(title)
	}
	file.Close()
	_ = os.RemoveAll(srcFile)

	// calculate sha256 sum
	sha256sum = fmt.Sprintf("%x", hash.Sum(nil))

	// Marshal the data into a json file.
	jsonData, err := json.Marshal(types.ZarfSplitPackageData{
		Count:     len(fileNames),
		Bytes:     fileSize,
		Sha256Sum: sha256sum,
	})
	if err != nil {
		return fmt.Errorf("unable to marshal the split package data: %w", err)
	}

	// write header file
	path = fmt.Sprintf("%s.part000", srcFile)
	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("unable to write the file %s: %w", path, err)
	}
	fileNames = append(fileNames, path)
	progressBar.Successf("Package split across %d files", len(fileNames))

	return nil
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

// IsTrashBin checks if the given directory path corresponds to an operating system's trash bin.
func IsTrashBin(dirPath string) bool {
	dirPath = filepath.Clean(dirPath)

	// Check if the directory path matches a Linux trash bin
	if strings.HasSuffix(dirPath, "/Trash") || strings.HasSuffix(dirPath, "/.Trash-1000") {
		return true
	}

	// Check if the directory path matches a macOS trash bin
	if strings.HasSuffix(dirPath, "./Trash") || strings.HasSuffix(dirPath, "/.Trashes") {
		return true
	}

	// Check if the directory path matches a Windows trash bin
	if strings.HasSuffix(dirPath, "\\$RECYCLE.BIN") {
		return true
	}

	return false
}

// IsHidden returns true if the given file name starts with a dot.
func IsHidden(name string) bool {
	return name[0] == dotCharacter
}

// GetDirSize walks through all files and directories in the provided path and returns the total size in bytes.
func GetDirSize(path string) (int64, error) {
	dirSize := int64(0)

	// Walk all files in the path
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

// IsDir returns true if the given path is a directory.
func IsDir(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// GetSHA256OfFile returns the SHA256 hash of the provided file.
func GetSHA256OfFile(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	return helpers.GetSHA256Hash(f)
}

// SHAsMatch returns an error if the SHA256 hash of the provided file does not match the expected hash.
func SHAsMatch(path, expected string) error {
	sha, err := GetSHA256OfFile(path)
	if err != nil {
		return err
	}
	if sha != expected {
		return fmt.Errorf("expected sha256 of %s to be %s, found %s", path, expected, sha)
	}
	return nil
}

// CreateReproducibleTarballFromDir creates a tarball from a directory with stripped headers
func CreateReproducibleTarballFromDir(dirPath, dirPrefix, tarballPath string) error {
	tb, err := os.Create(tarballPath)
	if err != nil {
		return fmt.Errorf("error creating tarball: %w", err)
	}
	defer tb.Close()

	tw := tar.NewWriter(tb)
	defer tw.Close()

	// Walk through the directory and process each file
	return filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		link := ""
		if info.Mode().Type() == os.ModeSymlink {
			link, err = os.Readlink(filePath)
			if err != nil {
				return fmt.Errorf("error reading symlink: %w", err)
			}
		}

		// Create a new header
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return fmt.Errorf("error creating tar header: %w", err)
		}

		// Strip non-deterministic header data
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""

		// Ensure the header's name is correctly set relative to the base directory
		name, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}
		name = filepath.Join(dirPrefix, name)
		name = filepath.ToSlash(name)
		header.Name = name

		// Write the header to the tarball
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("error writing header: %w", err)
		}

		// If it's a file, write its content
		if info.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("error opening file: %w", err)
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("error writing file to tarball: %w", err)
			}
		}

		return nil
	})
}
